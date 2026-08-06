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

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("CALL_ANALYTICS_URL"), "/")
	return &Client{
		baseURL:    baseURL,
		token:      os.Getenv("CALL_ANALYTICS_TOKEN"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

// AnalyzeCallRequest — запрос на классификацию одного звонка
type AnalyzeCallRequest struct {
	CallID     string `json:"call_id"`
	Transcript string `json:"transcript"`
}

// AnalyzeCallResult — результат классификации; поле "outcome" в AnalyticsJSON
// (например заинтересован/нейтрально/отказ/жалоба/перезвонить) — это ось, по
// которой UI строит распределение звонков за период.
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
