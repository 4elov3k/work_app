// Package docparse extracts text from uploaded documents (contracts, scans)
// by delegating to the Hermes ocr-extract service — a deterministic,
// LLM-free OCR/text-layer extraction endpoint — and then parses out the
// handful of fields work_app's contract form can prefill from it.
package docparse

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

// A multi-page scanned document can take well over a minute to OCR at
// 300 DPI (measured ~75s for a real 5-page contract scan) — the client
// timeout needs enough headroom for a handful of image-only pages.
const extractTimeout = 3 * time.Minute

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("OCR_SERVICE_URL"), "/")
	return &Client{
		baseURL: baseURL,
		token:   os.Getenv("OCR_SERVICE_TOKEN"),
		httpClient: &http.Client{
			Timeout: extractTimeout,
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

type Page struct {
	Page   int    `json:"page"`
	Method string `json:"method"`
	Text   string `json:"text"`
}

type ExtractResult struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	Pages    []Page `json:"pages"`
	Text     string `json:"text"`
}

// Extract sends the file to the ocr-extract service and returns its parsed text.
func (c *Client) Extract(ctx context.Context, filename string, data []byte) (*ExtractResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("OCR_SERVICE_URL is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/extract", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("building extract request: %w", err)
	}
	req.Header.Set("X-Filename", filename)
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ocr-extract: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ocr-extract response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errPayload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &errPayload)
		if errPayload.Error != "" {
			return nil, fmt.Errorf("ocr-extract: %s", errPayload.Error)
		}
		return nil, fmt.Errorf("ocr-extract returned HTTP %d", resp.StatusCode)
	}

	var result ExtractResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing ocr-extract response: %w", err)
	}
	return &result, nil
}
