// Package transcribe talks to the Hermes call-transcribe service, which
// runs faster-whisper locally — no LLM involved in turning audio into text.
// Per-call classification and period summaries are a separate concern (see
// internal/callreport) so a transcription outage never blocks the CDR sync
// itself, and vice versa.
package transcribe

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

// A multi-minute call recording can take a while to transcribe locally with
// faster-whisper — same rationale as docparse's OCR timeout. Bumped from an
// initial 3 minutes after real batch runs (4 concurrent calls, each
// analytics call also running its own CPU-heavy `hermes chat` subprocess on
// the same host) showed "context deadline exceeded" on requests that were
// still legitimately in progress, not stuck.
const transcribeTimeout = 6 * time.Minute

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("TRANSCRIBE_SERVICE_URL"), "/")
	return &Client{
		baseURL:    baseURL,
		token:      os.Getenv("TRANSCRIBE_SERVICE_TOKEN"),
		httpClient: &http.Client{Timeout: transcribeTimeout},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

type Result struct {
	Text string `json:"text"`
}

// Transcribe sends raw audio bytes to Hermes and returns the Whisper transcript.
func (c *Client) Transcribe(ctx context.Context, filename string, audio []byte) (*Result, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("TRANSCRIBE_SERVICE_URL is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transcribe", bytes.NewReader(audio))
	if err != nil {
		return nil, fmt.Errorf("building transcribe request: %w", err)
	}
	req.Header.Set("X-Filename", filename)
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling call-transcribe: %w", err)
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
			return nil, fmt.Errorf("call-transcribe: %s", errPayload.Error)
		}
		return nil, fmt.Errorf("call-transcribe returned HTTP %d", resp.StatusCode)
	}

	var result Result
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing call-transcribe response: %w", err)
	}
	return &result, nil
}
