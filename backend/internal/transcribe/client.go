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
}

// Transcribe dispatches to whichever format is configured (see Format) —
// FormatGPU tries the optional GPU box first and falls back to the CPU
// service on any failure there (including "box is off", which isn't a real
// error from the caller's point of view); FormatOpenRouter sends audio
// straight to OpenRouter with no whisper fallback at all.
func (c *Client) Transcribe(ctx context.Context, filename string, audio []byte) (*Result, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("транскрибация не настроена (формат %q)", c.format)
	}

	if c.format == FormatOpenRouter {
		return c.transcribeOpenRouter(ctx, audio)
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

// openRouterTranscribePrompt — same instruction set validated in the
// zvonari-sandbox experiments (2026-09-10/11): the original prompt's
// "(без речи)" rule only covered the zero-speech-at-all case, and on
// short/noisy calls the model was observed fabricating a full,
// plausible-sounding conversation instead of transcribing the
// (near-)absence of speech. The explicit anti-hallucination block below
// measurably reduced (but, per those tests, did not eliminate) the effect —
// see the accompanying hallucination-suspected check in
// zvonari.Service.transcribeOnly for the other half of the mitigation.
const openRouterTranscribePrompt = `Ты делаешь дословную расшифровку записи телефонного звонка на русском языке для компании «IQ-200» (агентство интернет-маркетинга: продвижение, привлечение клиентов, реклама, контекстная и таргетированная реклама, SEO, SMM, редизайн и разработка сайтов, лендинги, лидогенерация, заявки, конверсия, CRM, Яндекс Директ, стоимость заявки).

Один из собеседников — оператор/менеджер компании IQ-200, второй — потенциальный клиент. Компания «IQ-200» произносится как «АйКью-200» и похожими вариантами — не путай её название с посторонним словом или числом.

КРИТИЧЕСКИ ВАЖНО, ПРОЧТИ ПЕРЕД НАЧАЛОМ: транскрибируй ТОЛЬКО то, что реально произнесено и различимо на записи. Категорически запрещено придумывать, достраивать или галлюцинировать реплики — даже если по контексту тебе кажется понятным, что "обычно" говорят в таком звонке (про продвижение, рекламу, заявки и т.п.). Придуманная, но правдоподобная реплика — это грубая ошибка, гораздо хуже, чем короткая или пустая расшифровка. Если запись — это гудки, тишина, шум, обрыв связи или неразборчивое бормотание без узнаваемых слов, транскрипт должен закончиться в этом месте (или целиком быть "(без речи)", если разборчивых слов не было вообще) — не заполняй такие участки сочинённым диалогом о продвижении/рекламе/заявках, даже частично.

Сделай дословную расшифровку разговора, размечая реплики по говорящим построчно в формате:
Собеседник 1: <реплика>
Собеседник 2: <реплика>
...

Правила:
- Собеседник 1 — тот, кто заговорил первым (обычно оператор).
- Расшифровывай дословно, включая слова-паразиты и незаконченные фразы — не сокращай, не пересказывай, не суммаризируй, но и не досочиняй.
- В начале записи может быть тишина или гудки до ответа — это не повод писать "без речи": просто начинай расшифровку с первой реплики, без упоминания тишины/гудков перед ней.
- Автоответчик, голосовая почта, IVR-меню оператора связи ("Вызываемый абонент не отвечает...", "Вы позвонили в компанию...", "нажмите один" и т.п.) — это ТОЖЕ речь, которую нужно дословно расшифровать как реплику (Собеседник 1: <текст сообщения>), а не заменять ярлыком. Ярлык "автоответчик" сам по себе, без текста, — это потеря данных, так делать нельзя.
- Строку "(без речи)" пиши, если во всей записи буквально нет ни единого произнесённого слова — только гудки, тишина, шум или обрыв связи. Если есть хоть одна произнесённая фраза (человеком или автоответчиком) — расшифруй её и на этом остановись там, где реально обрывается различимая речь; "(без речи)" в этом случае не пиши, но и придумывать, что было сказано дальше, тоже нельзя.
- Ориентируйся на реальную длительность записи: беглая речь — это примерно 2-3 слова в секунду. Если твоя расшифровка выглядит заметно длиннее, чем могло бы уместиться в реальную продолжительность звонка, — это первый признак того, что ты начал сочинять; остановись и перепроверь, что каждая реплика реально была произнесена, а не додумана.
- Не добавляй от себя ничего, кроме самой расшифровки: без комментариев, оценок, markdown-разметки, заголовков.`

// transcribeOpenRouter sends audio straight to OpenRouter as a chat
// completion (input_audio content part) — same request shape used and
// validated in zvonari-sandbox. No segments/timecodes (flat text only, same
// as this package's Result always was), no spend cap enforced here — that's
// an OpenRouter-account-level concern, not this client's.
func (c *Client) transcribeOpenRouter(ctx context.Context, audio []byte) (*Result, error) {
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

	return &Result{Text: envelope.Choices[0].Message.Content, Engine: "openrouter"}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
