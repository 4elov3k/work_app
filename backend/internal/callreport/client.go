// Package callreport talks to the Hermes call-analytics service: ChatGPT-based
// classification of a single call's transcript, and a synthesized summary
// across a caller's calls for a period. Unlike transcribe (faster-whisper,
// local), this is the only place a transcript is ever sent to an LLM.
package callreport

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

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// analyzeHTTPTimeout must outlast call_analytics_server.py's own worst case
// for one /analyze-calls-batch request: the batch_timeout(analyzeBatchSize)
// budget for the initial combined attempt, PLUS — if that attempt fails —
// a full serial per-call fallback for every item in the batch (each
// individual analyze_call() call takes on the order of ~15-20s under the
// IQ-200 v1.2 rubric's per-step JSON schema). At analyzeBatchSize=25 that's
// roughly 810s + 25*20s ≈ 22 minutes worst case; 10 minutes (the previous
// value, sized for the old flat {category,outcome,note} schema) would
// abandon a batch that's still legitimately working, silently discarding
// its result and forcing a full retry next AnalyzeCalls run instead of
// just waiting a bit longer. If analyzeBatchSize changes, re-derive this.
const analyzeHTTPTimeout = 25 * time.Minute

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("CALL_ANALYTICS_URL"), "/")
	return &Client{
		baseURL:    baseURL,
		token:      os.Getenv("CALL_ANALYTICS_TOKEN"),
		httpClient: &http.Client{Timeout: analyzeHTTPTimeout},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

// healthPingTimeout bounds how long the /zvonari/health endpoint waits for
// this service before reporting it down — deliberately short, since this is
// polled from the UI and a hung analytics service shouldn't make the whole
// health check (and the page rendering it) hang too.
const healthPingTimeout = 2 * time.Second

// PingResult is shared by transcribe.Client's PingCPU/PingGPU and this
// method — Configured distinguishes "not set up" (grey in the UI) from
// Configured&&!OK ("down", red).
type PingResult struct {
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

// Ping checks hermes_call_analytics's /health endpoint — see
// zvonari-improvements.md, задача 7 (индикатор здоровья внешних сервисов).
func (c *Client) Ping(ctx context.Context) PingResult {
	if c.baseURL == "" {
		return PingResult{Configured: false}
	}
	ctx, cancel := context.WithTimeout(ctx, healthPingTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return PingResult{Configured: true, Error: err.Error()}
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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

// AnalyzeCallRequest — запрос на классификацию одного звонка. DurationSec/
// TalkTimeSec are the only real signal call_analytics_server.py has for its
// fraud_suspected heuristic ("автоответчик/меню звучит, и линия НЕ была
// сброшена вовремя") — without them the model can only guess "was this held
// too long" from how repetitive the transcript text looks, which is
// unreliable for short/garbled transcripts (see the 2026-08 investigation
// that added these fields: a 13s call with one truncated sentence had no
// textual signal either way and was scored fraud_suspected=false by
// omission, not by an actual timing judgement).
type AnalyzeCallRequest struct {
	CallID      string `json:"call_id"`
	Transcript  string `json:"transcript"`
	DurationSec int    `json:"duration_sec"`
	TalkTimeSec int    `json:"talk_time_sec"`
	// Direction ("inbound"/"outbound") lets the rubric tell a warm contact
	// (client called in, or replying to something they themselves started)
	// from a cold one — see the LPR-confirmation tiers in the regulation's
	// §5.2.1 (added 2026-09, "zvonari-lpr-criteria").
	Direction string `json:"direction"`
}

// AnalyzeCallResult — результат оценки звонка по скрипту IQ-200 (регламент
// v1.2); поле "outcome" в AnalyticsJSON — одно из 13 значений закрытого
// списка (например "Скрипт пройден до шага 6", "Срыв на шаге 1") — это ось,
// по которой UI строит распределение звонков за период. AnalyticsJSON также
// несёт разбор по шагам (steps), fraud_suspected и другие поля — см. схему
// в hermes/services/call_analytics_server.py.
type AnalyzeCallResult struct {
	AnalyticsJSON json.RawMessage `json:"analytics"`
}

func (c *Client) AnalyzeCall(ctx context.Context, req AnalyzeCallRequest) (*AnalyzeCallResult, error) {
	var result AnalyzeCallResult
	if err := c.post(ctx, "/analyze-call", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnalyzeCallsBatchRequest — классификация до нескольких десятков звонков
// одним запросом (один hermes-chat completion вместо одного на звонок) —
// то, что реально доминирует по времени в пере-анализе, это не сама
// классификация, а накладные расходы на процесс+сессию на каждый вызов.
type AnalyzeCallsBatchRequest struct {
	Calls []AnalyzeCallRequest `json:"calls"`
}

// AnalyzeCallsBatchResultItem — результат по одному звонку из батча,
// сопоставленный по call_id (сервис сопоставляет ответ модели по позиции
// внутри своего запроса и возвращает call_id и analytics обратно).
type AnalyzeCallsBatchResultItem struct {
	CallID        string          `json:"call_id"`
	AnalyticsJSON json.RawMessage `json:"analytics"`
}

type AnalyzeCallsBatchResult struct {
	Results []AnalyzeCallsBatchResultItem `json:"results"`
}

func (c *Client) AnalyzeCallsBatch(ctx context.Context, calls []AnalyzeCallRequest) (*AnalyzeCallsBatchResult, error) {
	var result AnalyzeCallsBatchResult
	if err := c.post(ctx, "/analyze-calls-batch", AnalyzeCallsBatchRequest{Calls: calls}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CallForReport — один звонок, передаваемый Hermes для синтеза отчёта
type CallForReport struct {
	CallID        string          `json:"call_id"`
	StartedAt     string          `json:"started_at"`
	Direction     string          `json:"direction"`
	Transcript    string          `json:"transcript"`
	AnalyticsJSON json.RawMessage `json:"analytics,omitempty"`
}

// GenerateReportRequest — запрос на сводный отчёт по звонарю за период
type GenerateReportRequest struct {
	CallerName  string          `json:"caller_name"`
	Period      string          `json:"period"`
	PeriodStart string          `json:"period_start"`
	PeriodEnd   string          `json:"period_end"`
	Calls       []CallForReport `json:"calls"`
}

// GenerateReportResult — сгенерированный Hermes отчёт
type GenerateReportResult struct {
	SummaryText string          `json:"summary_text"`
	MetricsJSON json.RawMessage `json:"metrics"`
}

func (c *Client) GenerateReport(ctx context.Context, req GenerateReportRequest) (*GenerateReportResult, error) {
	var result GenerateReportResult
	if err := c.post(ctx, "/generate-report", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) post(ctx context.Context, path string, payload interface{}, out interface{}) error {
	if !c.Configured() {
		return fmt.Errorf("CALL_ANALYTICS_URL is not configured")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling call-analytics %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading call-analytics response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errPayload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &errPayload)
		if errPayload.Error != "" {
			return fmt.Errorf("call-analytics: %s", errPayload.Error)
		}
		return fmt.Errorf("call-analytics returned HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing call-analytics response: %w", err)
	}
	return nil
}
