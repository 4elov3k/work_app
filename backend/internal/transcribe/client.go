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
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	// Optional: an on-LAN GPU box (see docs — a Windows machine running the
	// same transcribe_server.py with TRANSCRIBE_DEVICE=cuda), tried first
	// when configured. It isn't always powered on, so every call falls
	// straight back to baseURL (the always-on CPU service) on any failure —
	// there is no manual toggle, and no error is surfaced to the caller
	// just because the GPU box happened to be off.
	gpuBaseURL string
	gpuToken   string
	gpuClient  *http.Client
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

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("TRANSCRIBE_SERVICE_URL"), "/")
	gpuBaseURL := strings.TrimRight(os.Getenv("TRANSCRIBE_SERVICE_GPU_URL"), "/")
	return &Client{
		baseURL:    baseURL,
		token:      os.Getenv("TRANSCRIBE_SERVICE_TOKEN"),
		httpClient: &http.Client{Timeout: transcribeTimeout},
		gpuBaseURL: gpuBaseURL,
		gpuToken:   os.Getenv("TRANSCRIBE_SERVICE_GPU_TOKEN"),
		gpuClient:  &http.Client{Timeout: gpuTranscribeTimeout},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" || c.gpuBaseURL != ""
}

type Result struct {
	Text string `json:"text"`
	// Engine — какой сервис фактически ответил на этот запрос: "gpu" или
	// "cpu". Не приходит от Hermes (тот просто отвечает {text}) — это
	// Client сам знает, на какой из двух baseURL успешно достучался.
	Engine string `json:"-"`
}

// Transcribe tries the optional GPU box first (if configured), and falls
// back to the CPU service on any failure there — including "box is off",
// which isn't a real error from the caller's point of view.
func (c *Client) Transcribe(ctx context.Context, filename string, audio []byte) (*Result, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("TRANSCRIBE_SERVICE_URL is not configured")
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
