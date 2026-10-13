// Package transcribe talks to the Hermes call-transcribe service
// (faster-whisper, local) or, when TRANSCRIBE_FORMAT=openrouter, sends
// audio straight to OpenRouter instead — see Format below.  Per-call
// classification and period summaries are a separate concern (see
// internal/callreport) so a transcription outage never blocks the CDR sync
// itself, and vice versa.
package transcribe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Format picks which transcription path Transcribe() uses — set once at
// startup via TRANSCRIBE_FORMAT, no runtime/per-call switch. The two
// formats are deliberately independent: FormatGPU never falls back to
// FormatOpenRouter or vice versa, so a misconfigured/rate-limited
// OpenRouter key can't silently start eating the whisper boxes' traffic
// (or the reverse — a whisper outage silently starting to spend OpenRouter
// budget). Picking the format is a manual, conscious choice, not automatic.
type Format string

const (
	FormatGPU        Format = "gpu" // default — existing gpu-then-cpu whisper path, unchanged
	FormatOpenRouter Format = "openrouter"
)

type Client struct {
	format Format

	baseURL    string
	token      string
	httpClient *http.Client

	// Optional: an on-LAN GPU box (see docs — a Windows machine running the
	// same transcribe_server.py with TRANSCRIBE_DEVICE=cuda), tried first
	// when configured. It isn't always powered on, so every call falls
	// straight back to baseURL (the always-on CPU service) on any failure —
	// there is no manual toggle, and no error is surfaced to the caller
	// just because the GPU box happened to be off. Only relevant when
	// format == FormatGPU.
	gpuBaseURL string
	gpuToken   string
	gpuClient  *http.Client

	// OpenRouter — only relevant when format == FormatOpenRouter. Separate
	// budget/account from callreport's LLM usage; no automatic fallback to
	// the whisper boxes on failure (see Format's doc comment) — a failed
	// OpenRouter request is a real error to the caller, same as a failed
	// whisper request is in FormatGPU.
	openRouterAPIKey string
	openRouterModel  string
	openRouterClient *http.Client
}

// A multi-minute call recording can take a while to transcribe locally with
// faster-whisper — same rationale as docparse's OCR timeout. Bumped from an
// initial 3 minutes after real batch runs (4 concurrent calls, each
// analytics call also running its own CPU-heavy `hermes chat` subprocess on
// the same host) showed "context deadline exceeded" on requests that were
// still legitimately in progress, not stuck.
const transcribeTimeout = 6 * time.Minute

// 90s was tuned assuming "GPU transcription finishes in seconds" — true for
// a typical few-minute call, but measured false for a real ~16-minute call,
// which was still legitimately transcribing on the GPU box when this fired,
// silently falling back to a much slower CPU pass instead. Still meant to
// fail fast if the GPU box is unreachable (TCP refused/no-route return
// almost instantly, long before this budget matters) or genuinely hung —
// just no longer assumes every call is short.
const gpuTranscribeTimeout = 5 * time.Minute

// openRouterTranscribeTimeout — a large call recording base64-encoded plus
// a real generation pass (reasoning-capable models especially) can run a
// couple of minutes; same order of magnitude as gpuTranscribeTimeout.
const openRouterTranscribeTimeout = 5 * time.Minute

const openRouterChatURL = "https://openrouter.ai/api/v1/chat/completions"

func NewFromEnv() *Client {
	format := Format(strings.ToLower(strings.TrimSpace(os.Getenv("TRANSCRIBE_FORMAT"))))
	if format == "" {
		format = FormatGPU
	}
	baseURL := strings.TrimRight(os.Getenv("TRANSCRIBE_SERVICE_URL"), "/")
	gpuBaseURL := strings.TrimRight(os.Getenv("TRANSCRIBE_SERVICE_GPU_URL"), "/")
	openRouterModel := os.Getenv("TRANSCRIBE_OPENROUTER_MODEL")
	if openRouterModel == "" {
		openRouterModel = "google/gemini-2.5-flash-lite"
	}
	return &Client{
		format: format,

		baseURL:    baseURL,
		token:      os.Getenv("TRANSCRIBE_SERVICE_TOKEN"),
		httpClient: &http.Client{Timeout: transcribeTimeout},
		gpuBaseURL: gpuBaseURL,
		gpuToken:   os.Getenv("TRANSCRIBE_SERVICE_GPU_TOKEN"),
		gpuClient:  &http.Client{Timeout: gpuTranscribeTimeout},

		openRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		openRouterModel:  openRouterModel,
		openRouterClient: &http.Client{Timeout: openRouterTranscribeTimeout},
	}
}

func (c *Client) Configured() bool {
	if c.format == FormatOpenRouter {
		return c.openRouterAPIKey != ""
	}
	return c.baseURL != "" || c.gpuBaseURL != ""
}

// IsOpenRouter reports whether TRANSCRIBE_FORMAT=openrouter is active — see
// zvonari.Service's concurrency choice, which is network-bound-vs-CPU-bound
// dependent (maxConcurrentProcessing's whisper-tuned value of 1 doesn't
// apply to remote OpenRouter calls).
func (c *Client) IsOpenRouter() bool {
	return c.format == FormatOpenRouter
}

// healthPingTimeout bounds how long the /zvonari/health endpoint waits for
// each transcribe backend before reporting it down.
const healthPingTimeout = 2 * time.Second

// PingResult mirrors callreport.Client's — Configured distinguishes "not set
// up" (grey in the UI, e.g. no GPU box) from Configured&&!OK ("down", red).
type PingResult struct {
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

func pingHealth(baseURL, token string) PingResult {
	if baseURL == "" {
		return PingResult{Configured: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthPingTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return PingResult{Configured: true, Error: err.Error()}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: healthPingTimeout}).Do(req)
	if err != nil {
		return PingResult{Configured: true, Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return PingResult{Configured: true, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return PingResult{Configured: true, OK: true}
}

// PingCPU checks the always-on CPU transcribe service's /health endpoint —
// see zvonari-improvements.md, задача 7. Meaningless in FormatOpenRouter
// (reports not-configured, not down — there's no CPU box in that format).
func (c *Client) PingCPU() PingResult {
	if c.format != FormatGPU {
		return PingResult{Configured: false}
	}
	return pingHealth(c.baseURL, c.token)
}

// PingGPU checks the optional on-LAN GPU box's /health endpoint — reports
// Configured=false (not down) when TRANSCRIBE_SERVICE_GPU_URL isn't set,
// since an unconfigured GPU box isn't an outage. Meaningless in
// FormatOpenRouter, same reasoning as PingCPU.
func (c *Client) PingGPU() PingResult {
	if c.format != FormatGPU {
		return PingResult{Configured: false}
	}
	return pingHealth(c.gpuBaseURL, c.gpuToken)
}

// PingOpenRouter reports whether TRANSCRIBE_FORMAT=openrouter is active and
// an API key is set — no real HTTP request (unlike PingCPU/PingGPU's
// /health call), since there's no free health-check endpoint on OpenRouter
// and hitting chat/completions just to check "is it up" would cost real
// money for no reason.
func (c *Client) PingOpenRouter() PingResult {
	if c.format != FormatOpenRouter {
		return PingResult{Configured: false}
	}
	if c.openRouterAPIKey == "" {
		return PingResult{Configured: true, Error: "OPENROUTER_API_KEY is not set"}
	}
	return PingResult{Configured: true, OK: true}
}

type Result struct {
	Text string `json:"text"`
	// Engine — какой сервис фактически ответил на этот запрос: "gpu",
	// "cpu" или "openrouter". Не приходит от Hermes (тот просто отвечает
	// {text}) — это Client сам знает, какой формат/backend использовался.
	Engine string `json:"-"`

	// Fraud — заполнен только в FormatOpenRouter: тот же запрос, что
	// расшифровывает аудио, заодно определяет фрод-сигналы (см.
	// computeFraudPrecheck) — одним вызовом вместо двух отдельных
	// (транскрибация + отдельный hermes-анализ), см. обсуждение экономии
	// 2026-09-29. В FormatGPU/FormatCPU остаётся нулевым: там нет LLM,
	// который мог бы это определить.
	Fraud FraudPrecheck `json:"-"`
}

// FraudPrecheck — фрод-сигнал, определённый на этапе транскрибации, по
// аудио, а не по уже готовому тексту (в отличие от полного hermes-анализа).
// Это ПРЕДВАРИТЕЛЬНЫЙ, дешёвый сигнал — окончательное решение об
// исключении из оплаты принимает человек (см. fraud_review_status в БД),
// не эта структура сама по себе.
type FraudPrecheck struct {
	Suspected bool   `json:"fraud_suspected"`
	Check     string `json:"fraud_check"`
	Quote     string `json:"disqualifying_phrase_quote,omitempty"`
	Position  string `json:"disqualifying_phrase_position,omitempty"`
}

// Transcribe dispatches to whichever format is configured (see Format) —
// FormatGPU tries the optional GPU box first and falls back to the CPU
// service on any failure there (including "box is off", which isn't a real
// error from the caller's point of view); FormatOpenRouter sends audio
// straight to OpenRouter with no whisper fallback at all. talkTimeSec is
// only used by FormatOpenRouter's fraud precheck (see computeFraudPrecheck)
// — it comes from the PBX CDR record, already known before transcription
// ever runs, so no separate lookup is needed here.
func (c *Client) Transcribe(ctx context.Context, filename string, audio []byte, talkTimeSec int) (*Result, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("транскрибация не настроена (формат %q)", c.format)
	}

	if c.format == FormatOpenRouter {
		return c.transcribeOpenRouter(ctx, audio, talkTimeSec)
	}

	if c.gpuBaseURL != "" {
		result, err := c.transcribeAt(ctx, c.gpuClient, c.gpuBaseURL, c.gpuToken, filename, audio)
		if err == nil {
			result.Engine = "gpu"
			return result, nil
		}
		if c.baseURL == "" {
			return nil, err
		}
		log.Printf("transcribe: GPU service unavailable, falling back to CPU: %v", err)
	}

	result, err := c.transcribeAt(ctx, c.httpClient, c.baseURL, c.token, filename, audio)
	if err != nil {
		return nil, err
	}
	result.Engine = "cpu"
	return result, nil
}

func (c *Client) transcribeAt(ctx context.Context, client *http.Client, baseURL, token, filename string, audio []byte) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/transcribe", bytes.NewReader(audio))
	if err != nil {
		return nil, fmt.Errorf("building transcribe request: %w", err)
	}
	req.Header.Set("X-Filename", filename)
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling call-transcribe (%s): %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading call-transcribe response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errPayload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &errPayload)
		if errPayload.Error != "" {
			return nil, fmt.Errorf("call-transcribe (%s): %s", baseURL, errPayload.Error)
		}
		return nil, fmt.Errorf("call-transcribe (%s) returned HTTP %d", baseURL, resp.StatusCode)
	}

	var result Result
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing call-transcribe response: %w", err)
	}
	return &result, nil
}

// openRouterTranscribePrompt — расшифровка И фрод-предфильтр ОДНИМ вызовом
// (2026-09-29, см. обсуждение экономии — раньше это были транскрибация +
// отдельный hermes-анализ, т.е. два платных вызова на звонок вместо одного,
// и hermes при этом определял фрод по уже готовому ТЕКСТУ, а не слыша само
// аудио). Текстовые правила расшифровки не менялись, валидированы ранее
// (zvonari-sandbox, 2026-09-10/11). Фрод-часть переносит уже провалидированную
// на 327 звонках логику из hermes/services/call_analytics_server.py
// (ANALYSIS_RUBRIC + compute_fraud_suspected) — катал ог дисквалифицирующих
// фраз (включая саморепрезентацию бота-ассистента и рекламу "номер в
// свободной продаже"), различение филлера "оставайтесь на линии" от
// содержательной навигации. Не переносит is_looping_or_repeating — в hermes
// эта проверка была полностью вытеснена позицией фразы (см. её
// compute_fraud_suspected, 2026-09-29: "not used as an independent signal
// here"), так что переносить нечего.
//
// Ответ — JSON-объект (response_format json_object в запросе ниже), а не
// голый текст, как раньше: transcript-поле содержит ту же дословную
// расшифровку, что и прежде, остальные поля — фрод-сигнал. Причина смены
// формата, а не двух отдельных вызовов — экономия токенов на системном
// промпте (одна встроенная инструкция вместо двух отдельных обращений).
const openRouterTranscribePrompt = `Ты делаешь дословную расшифровку записи телефонного звонка на русском языке для компании «IQ-200» (агентство интернет-маркетинга: продвижение, привлечение клиентов, реклама, контекстная и таргетированная реклама, SEO, SMM, редизайн и разработка сайтов, лендинги, лидогенерация, заявки, конверсия, CRM, Яндекс Директ, стоимость заявки), и ОДНОВРЕМЕННО определяешь по этой же записи признаки фрода (автоответчик/IVR/бот, за которым не последовало реального разговора с человеком).

Один из собеседников — оператор/менеджер компании IQ-200, второй — потенциальный клиент. Компания «IQ-200» произносится как «АйКью-200» и похожими вариантами — не путай её название с посторонним словом или числом.

=== ЧАСТЬ 1: РАСШИФРОВКА (поле "transcript") ===

КРИТИЧЕСКИ ВАЖНО, ПРОЧТИ ПЕРЕД НАЧАЛОМ: транскрибируй ТОЛЬКО то, что реально произнесено и различимо на записи. Категорически запрещено придумывать, достраивать или галлюцинировать реплики — даже если по контексту тебе кажется понятным, что "обычно" говорят в таком звонке (про продвижение, рекламу, заявки и т.п.). Придуманная, но правдоподобная реплика — это грубая ошибка, гораздо хуже, чем короткая или пустая расшифровка. Если запись — это гудки, тишина, шум, обрыв связи или неразборчивое бормотание без узнаваемых слов, транскрипт должен закончиться в этом месте (или целиком быть "(без речи)", если разборчивых слов не было вообще) — не заполняй такие участки сочинённым диалогом о продвижении/рекламе/заявках, даже частично.

Сделай дословную расшифровку разговора, размечая реплики по говорящим построчно в формате:
Собеседник 1: <реплика>
Собеседник 2: <реплика>
...

Правила:
- Собеседник 1 и Собеседник 2 — это метки ТОЛЬКО для двух живых людей в разговоре (оператор IQ-200 и представитель компании-клиента). Собеседник 1 — тот из них, кто заговорил первым.
- Расшифровывай дословно, включая слова-паразиты и незаконченные фразы — не сокращай, не пересказывай, не суммаризируй, но и не досочиняй.
- В начале записи может быть тишина или гудки до ответа — это не повод писать "без речи": просто начинай расшифровку с первой реплики, без упоминания тишины/гудков перед ней.
- Автоответчик, голосовая почта, IVR-меню оператора связи ("Вызываемый абонент не отвечает...", "Вы позвонили в компанию...", "нажмите один" и т.п.) — это ТОЖЕ речь, которую нужно дословно расшифровать, но отдельной меткой "Автоответчик:" (например: Автоответчик: <текст сообщения>), а НЕ как "Собеседник 1"/"Собеседник 2" — даже если это автоматическое сообщение звучит первым по времени, метку "Собеседник 1" получает первый из живых людей, заговоривший после него, а не само сообщение. Не заменяй текст автоответчика ярлыком без содержания — ярлык "автоответчик" сам по себе, без текста, — это потеря данных, так делать нельзя.
- Строку "(без речи)" пиши, если во всей записи буквально нет ни единого произнесённого слова — только гудки, тишина, шум или обрыв связи. Если есть хоть одна произнесённая фраза (человеком или автоответчиком) — расшифруй её и на этом остановись там, где реально обрывается различимая речь; "(без речи)" в этом случае не пиши, но и придумывать, что было сказано дальше, тоже нельзя.
- Ориентируйся на реальную длительность записи: беглая речь — это примерно 2-3 слова в секунду. Если твоя расшифровка выглядит заметно длиннее, чем могло бы уместиться в реальную продолжительность звонка, — это первый признак того, что ты начал сочинять; остановись и перепроверь, что каждая реплика реально была произнесена, а не додумана.

=== ЧАСТЬ 2: ФРОД-СИГНАЛ (остальные поля) ===

Определи is_answering_machine_or_menu: true/false — был ли на протяжении ВСЕЙ доступной записи слышен только автоответчик/голосовое меню/IVR/бот-ассистент, без единой реплики живого человека, обращённой к оператору. Как только в записи появляется реальная реплика человека (даже одно слово) — is_answering_machine_or_menu = false, даже если разговору предшествовало меню.

К автоответчику/меню относится и разговорный бот-ассистент, выдающий себя за секретаря (даже если звучит естественно и ведёт диалог): "я голосовой/виртуальный ассистент", "меня зовут Ния/Мия" (или другое явно небытовое имя бота), "передам это абоненту", "вы робот или человек?" — если все реплики собеседника сводятся к приёму и ретрансляции сообщения для отсутствующего абонента, а не к содержательному ответу от своего лица.

Если is_answering_machine_or_menu = true, найди в записи первую фразу, которая ОДНОЗНАЧНО говорит, что живого человека на линии не будет — "абонент не может ответить/недоступен/выключен/вне зоны действия сети", приглашение оставить голосовое сообщение, сообщение оператора связи об ограничении вызовов, самопрезентация бота-ассистента (см. выше), объявление о статусе самого номера ("номер находится в свободной продаже", "номер принадлежит сервису Х") — это disqualifying_phrase_quote. Голосовое меню/маршрутизация ("нажмите 1 для отдела продаж"), просьба подождать/оставаться на линии САМИ ПО СЕБЕ такой фразой не являются — это часть нормального процесса дозвона, если только не сопровождаются реальным содержанием (название компании, конкретные пункты меню). Голый филлер без содержания ("оставайтесь на линии", "продолжаем дозваниваться", "соединение не установлено, подождите") перед отказной фразой не считается содержательной навигацией.

Если фраза найдена — определи disqualifying_phrase_position ("начало"/"конец"):
- "начало" — фраза звучит одной из первых, без содержательного меню/навигации перед ней (включая случай, когда перед ней только голый филлер без содержания, см. выше).
- "конец" — фразе предшествует СОДЕРЖАТЕЛЬНОЕ меню/приветствие (название компании, конкретные пункты меню) — то есть часть звонка была законной навигацией.
Если такой фразы нет вообще (меню/ожидание без финального отказа) — disqualifying_phrase_quote = null, disqualifying_phrase_position = null.

Если расшифровка повреждена/оборвана настолько, что нельзя уверенно определить, был ли на линии человек — НЕ домысливай в пользу оператора любой ценой: это предварительный сигнал для последующей проверки человеком, а не финальное решение об исключении из оплаты, поэтому при реальном сомнении ставь is_answering_machine_or_menu = true с кратким пояснением в disqualifying_phrase_quote — лучше лишний раз попадёт в список на проверку, чем реальный фрод будет пропущен.

=== ФОРМАТ ОТВЕТА ===
Верни ТОЛЬКО JSON-объект, без markdown-обёртки и пояснений, строго такой формы:
{
  "transcript": "<дословная расшифровка из части 1>",
  "is_answering_machine_or_menu": true или false,
  "disqualifying_phrase_quote": "<цитата>" или null,
  "disqualifying_phrase_position": "начало" или "конец" или null
}`

// transcribeOpenRouter sends audio straight to OpenRouter as a chat
// completion (input_audio content part) — same request shape used and
// validated in zvonari-sandbox, plus response_format json_object so the
// combined transcript+fraud response parses reliably (see
// openRouterTranscribePrompt's doc comment for why fraud detection moved
// in here). No segments/timecodes (flat text only, same as this package's
// Result always was), no spend cap enforced here — that's an
// OpenRouter-account-level concern, not this client's.
func (c *Client) transcribeOpenRouter(ctx context.Context, audio []byte, talkTimeSec int) (*Result, error) {
	b64 := base64.StdEncoding.EncodeToString(audio)
	reqBody, err := json.Marshal(map[string]any{
		"model": c.openRouterModel,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": openRouterTranscribePrompt},
					{"type": "input_audio", "input_audio": map[string]string{"data": b64, "format": "mp3"}},
				},
			},
		},
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding OpenRouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterChatURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("building OpenRouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.openRouterAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.openRouterClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OpenRouter response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("unexpected OpenRouter response shape: %s", truncate(string(body), 500))
	}
	content := envelope.Choices[0].Message.Content

	var parsed struct {
		Transcript                  string `json:"transcript"`
		IsAnsweringMachineOrMenu    bool   `json:"is_answering_machine_or_menu"`
		DisqualifyingPhraseQuote    string `json:"disqualifying_phrase_quote"`
		DisqualifyingPhrasePosition string `json:"disqualifying_phrase_position"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil || parsed.Transcript == "" {
		// Деградация без потери транскрипта: если модель вернула не тот JSON,
		// который мы просили (или вообще голый текст — response_format не
		// гарантия), лучше сохранить content как есть в виде транскрипта без
		// фрод-сигнала, чем потерять расшифровку целиком, как это уже было в
		// инциденте 2026-09-28 с другой ошибкой (см. maxConcurrentProcessing в
		// zvonari/service.go). FraudPrecheck остаётся нулевым — вызывающий код
		// должен уметь работать с пустым Check ("" != любой реальный код).
		log.Printf("transcribe: OpenRouter response wasn't the expected JSON shape, falling back to raw content as transcript: %s", truncate(content, 300))
		return &Result{Text: content, Engine: "openrouter"}, nil
	}

	fraud := computeFraudPrecheck(parsed.IsAnsweringMachineOrMenu, talkTimeSec, parsed.DisqualifyingPhrasePosition, parsed.DisqualifyingPhraseQuote)
	return &Result{Text: parsed.Transcript, Engine: "openrouter", Fraud: fraud}, nil
}

// computeFraudPrecheck — детерминированное решение fraud_suspected, портировано
// из hermes/services/call_analytics_server.py:compute_fraud_suspected
// (2026-09-29 редизайн: позиция фразы решает всё, is_looping_or_repeating
// больше не отдельная ветка — см. её докстринг для истории и валидации на
// 327 звонках). Логика 1-в-1 с hermes, включая ветку "нет чёткой отказной
// фразы = не фрод" ниже — это НЕ то место для уклона в recall: этот случай
// (is_answering_machine_or_menu=true, но фраза не найдена — обычно
// department-routing IVR-меню без финального отказа) на тех же 327 звонках
// дал 6 из 6 ложных срабатываний и ни одного верного, когда проверялся как
// отдельное правило — переворачивать его в "кандидат" означало бы затопить
// очередь на проверку легитимными "ещё не дозвонился до человека" звонками
// без единого выигрыша в полноте. Уклон в recall, о котором просил
// пользователь, уже учтён на уровне промпта (openRouterTranscribePrompt —
// инструкция "при реальном сомнении в повреждённой записи ставь true"), не
// здесь: там модель принимает решение по факту записи, здесь код просто
// детерминированно применяет уже принятое решение.
func computeFraudPrecheck(isAnsweringMachineOrMenu bool, talkTimeSec int, position, quote string) FraudPrecheck {
	if !isAnsweringMachineOrMenu {
		return FraudPrecheck{Suspected: false, Check: "not_answering_machine"}
	}
	if talkTimeSec > 0 && talkTimeSec < 10 {
		return FraudPrecheck{Suspected: false, Check: "fast_hangup_under_10s"}
	}
	if position == "конец" {
		return FraudPrecheck{Suspected: false, Check: "disqualifying_phrase_at_end"}
	}
	if position == "начало" {
		return FraudPrecheck{Suspected: true, Check: "disqualifying_phrase_at_start", Quote: quote, Position: position}
	}
	return FraudPrecheck{Suspected: false, Check: "no_disqualifying_phrase_found"}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
