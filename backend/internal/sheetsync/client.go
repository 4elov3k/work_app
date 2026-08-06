// Package sheetsync talks to the Hermes sheets-sync service, which is the
// only thing with real Google Sheets access — work_app's backend has no
// Google credentials of its own. This client only calls two endpoints:
// "what's the next act number" and "register this act's row" — the actual
// find-last-row/write-without-shifting-columns logic lives entirely on the
// sheets-sync side (see hermes/data/skills/.../act_number_sync.py), so a
// work_app change here can never accidentally corrupt the real spreadsheet.
package sheetsync

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
	baseURL := strings.TrimRight(os.Getenv("SHEETS_SYNC_URL"), "/")
	return &Client{
		baseURL: baseURL,
		token:   os.Getenv("SHEETS_SYNC_TOKEN"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

type NextNumberResult struct {
	Row    int    `json:"row"`
	Number string `json:"number"`
}

func (c *Client) NextActNumber(ctx context.Context) (*NextNumberResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("SHEETS_SYNC_URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/next-act-number", nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling sheets-sync: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(body, resp.StatusCode)
	}

	var result NextNumberResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing sheets-sync response: %w", err)
	}
	return &result, nil
}

type WriteRowResult struct {
	Row           int    `json:"row"`
	Number        string `json:"number"`
	UpdatedCells  int    `json:"updated_cells"`
}

// RegisterAct writes a new row for a just-created act. The sheets-sync
// service recomputes the next row/number itself at write time — it does
// NOT trust a caller-supplied number — so this always reflects the true
// current state of the sheet, not whatever GetNextActNumber returned
// earlier (which could be stale if time passed between the two calls).
func (c *Client) RegisterAct(ctx context.Context, contract, date string) (*WriteRowResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("SHEETS_SYNC_URL is not configured")
	}
	payload, err := json.Marshal(map[string]string{"contract": contract, "date": date})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/write-act-row", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling sheets-sync: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(body, resp.StatusCode)
	}

	var result WriteRowResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing sheets-sync response: %w", err)
	}
	return &result, nil
}

func (c *Client) authorize(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func apiError(body []byte, status int) error {
	var errPayload struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &errPayload)
	if errPayload.Error != "" {
		return fmt.Errorf("sheets-sync: %s", errPayload.Error)
	}
	return fmt.Errorf("sheets-sync returned HTTP %d", status)
}
