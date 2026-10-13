// Package analysis talks to OpenRouter directly for the 4-criteria call
// scoring (listening/goal_understanding/questions/objection_handling) —
// 2026-09-29, replaces going through Hermes's call-analytics service for
// this specific job, for the same reason transcribe.Client talks to
// OpenRouter directly for transcription+fraud-precheck: one fewer hop, one
// fewer service to keep alive, no reason to round-trip through a Python
// container that just re-forwards to the same OpenRouter endpoint. The
// rubric/prompt was validated in zvonari-sandbox first (prompt
// "Стажировочный день") and the model (gpt-6-luna) was chosen there too —
// see work_app session notes for the 3-call comparison against
// gemini-2.5-flash. callreport.Client (Hermes) is still used for the
// period-report synthesis (a different, free-text-summary job), just not
// for this one anymore.
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const openRouterChatURL = "https://openrouter.ai/api/v1/chat/completions"

// analyzeTimeout — a single criteria-scoring call is a small structured-JSON
// request, same order of magnitude as transcribe's per-call OpenRouter
// timeout, not the multi-minute budget a whole-period report synthesis
// needs (that's still callreport.GenerateReport, unaffected by this
// package).
const analyzeTimeout = 2 * time.Minute

type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewFromEnv() *Client {
	model := os.Getenv("ANALYSIS_OPENROUTER_MODEL")
	if model == "" {
		model = "openai/gpt-6-luna"
	}
	return &Client{
		apiKey:     os.Getenv("OPENROUTER_API_KEY"),
		model:      model,
		httpClient: &http.Client{Timeout: analyzeTimeout},
	}
}

func (c *Client) Configured() bool {
	return c.apiKey != ""
}

// PingResult mirrors transcribe.PingResult/callreport.PingResult — same
// Configured/OK/Error shape the health-dot UI already expects.
type PingResult struct {
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

// Ping reports whether the API key is set — no real HTTP request, same
// reasoning as transcribe.Client.PingOpenRouter: there's no free health-check
// endpoint on OpenRouter, and a real chat/completions call just to check
// "is it up" would cost real money for no reason.
func (c *Client) Ping() PingResult {
	if c.apiKey == "" {
		return PingResult{Configured: true, Error: "OPENROUTER_API_KEY is not set"}
	}
	return PingResult{Configured: true, OK: true}
}

// criteriaPrompt — validated in zvonari-sandbox (prompt "Стажировочный
// день", 2026-09-17/18) before being wired in here 2026-09-29. Identical
// wording to hermes/services/call_analytics_server.py's ANALYSIS_RUBRIC —
// keep the two in sync if this changes; hermes's copy stays live for
// sandbox-style experimentation, this one is what work_app's own pipeline
// actually calls.
const criteriaPrompt = `Ты — аналитик отдела продаж компании IQ-200, оценивающий звонок оператора холодного обзвона. Конечная цель оператора — назначить предметную встречу с ЛПР (лицом, принимающим решения).

Оцени звонок строго по 4 критериям. По каждому дай met=true/false/null и обоснование с конкретной цитатой или пересказом места из звонка:

1. listening — Умение слушать: прежде чем презентовать компанию, оператор сначала внимательно слушает собеседника и дальше использует его же формулировки, подстраивается под то, что волнует именно этого человека, а не жёстко держится скрипта и не говорит только про компанию.
2. goal_understanding — Понимание конечной цели разговора: оператор постоянно держит в уме цель "назначить предметную встречу с ЛПР", и каждый шаг диалога логически ведёт к ней (например, уточняет, кто ЛПР, прежде чем договариваться о встрече; если обещает отправить информацию — уточняет, кому и обозначает следующий шаг).
3. questions — Умение задавать вопросы: использует открытые вопросы, чтобы разговорить собеседника и собрать информацию, и уточняющие — чтобы докопаться до сути возражения, не спорит и не давит своей позицией, а ведёт к открытому диалогу.
4. objection_handling — Умение работать с возражениями: услышав отказ, не сдаётся сразу, уточняющими вопросами выясняет истинную причину, сначала соглашается с возражением клиента, а затем мягко аргументирует пользу встречи именно в его ситуации, используя его же слова.

Если для какого-то критерия в звонке просто не было подходящей ситуации (например, клиент вообще не возражал) — ставь met=null и поясни это в comment; такой критерий не засчитывается ни как выполненный, ни как провальный.

Длительность звонка: %d сек.

Расшифровка звонка:
"""
%s
"""

Верни ЧИСТЫЙ JSON без пояснений и markdown-обёртки, строго такой формы:
{"criteria": {
  "listening": {"met": true/false/null, "evidence": "...", "comment": "..."},
  "goal_understanding": {"met": true/false/null, "evidence": "...", "comment": "..."},
  "questions": {"met": true/false/null, "evidence": "...", "comment": "..."},
  "objection_handling": {"met": true/false/null, "evidence": "...", "comment": "..."}
},
"score": "X/N",
"summary": "краткое резюме сильных и слабых сторон оператора в этом звонке",
"recommendation": "что конкретно улучшить в следующих звонках"}

score — количество критериев с met=true через дробь от числа применимых критериев (met=null исключается и из числителя, и из знаменателя).`

// Criterion — one of the 4 axes' scored result.
type Criterion struct {
	Met      *bool  `json:"met"`
	Evidence string `json:"evidence"`
	Comment  string `json:"comment"`
}

// Result — the stored analytics_json shape for a criteria-scored call.
// Identical field names/shape to hermes's _normalize_analytics output (see
// call_analytics_server.py) on purpose — a call scored via either path
// looks the same to the frontend.
type Result struct {
	Criteria struct {
		Listening         Criterion `json:"listening"`
		GoalUnderstanding Criterion `json:"goal_understanding"`
		Questions         Criterion `json:"questions"`
		ObjectionHandling Criterion `json:"objection_handling"`
	} `json:"criteria"`
	Score          string `json:"score"`
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
}

// AnalyzeCall scores one call's transcript against the 4 criteria. Never
// trusts the model's own `score` string — recomputes it from the normalized
// criteria, same principle hermes's _normalize_analytics uses (don't trust
// arithmetic the model did in the same breath as the qualitative read).
func (c *Client) AnalyzeCall(ctx context.Context, transcript string, durationSec int) (*Result, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("анализ по критериям не настроен (OPENROUTER_API_KEY пуст)")
	}
	prompt := fmt.Sprintf(criteriaPrompt, durationSec, transcript)

	reqBody, err := json.Marshal(map[string]any{
		"model":           c.model,
		"messages":        []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]string{"type": "json_object"},
		"max_tokens":      4000,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding OpenRouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterChatURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("building OpenRouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
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
	content := strings.TrimSpace(envelope.Choices[0].Message.Content)

	var parsed struct {
		Criteria map[string]struct {
			Met      *bool  `json:"met"`
			Evidence string `json:"evidence"`
			Comment  string `json:"comment"`
		} `json:"criteria"`
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("model did not return the expected JSON shape: %s", truncate(content, 500))
	}

	result := &Result{Summary: parsed.Summary, Recommendation: parsed.Recommendation}
	metTrue, applicable := 0, 0
	assign := func(key string) Criterion {
		raw, ok := parsed.Criteria[key]
		if !ok {
			return Criterion{}
		}
		if raw.Met != nil {
			applicable++
			if *raw.Met {
				metTrue++
			}
		}
		return Criterion{Met: raw.Met, Evidence: raw.Evidence, Comment: raw.Comment}
	}
	result.Criteria.Listening = assign("listening")
	result.Criteria.GoalUnderstanding = assign("goal_understanding")
	result.Criteria.Questions = assign("questions")
	result.Criteria.ObjectionHandling = assign("objection_handling")
	result.Score = fmt.Sprintf("%d/%d", metTrue, applicable)

	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
