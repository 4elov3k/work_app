// Package sheetsync talks to the Hermes sheets-sync service, which is the
// only thing with real Google Sheets access — work_app's backend has no
// Google credentials of its own. This client calls "what's the next
// act/invoice number" and "register this act's/invoice's row" — the actual
// find-last-row/write-without-shifting-columns logic lives entirely on the
// sheets-sync side (see hermes/data/skills/.../act_number_sync.py and
// invoice_number_sync.py), so a work_app change here can never accidentally
// corrupt the real spreadsheet.
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
	return c.nextNumber(ctx, "/next-act-number")
}

// NextInvoiceNumber reads the next invoice number from the invoice-numbering
// registry sheet — a separate spreadsheet from the acts registry (see
// SHEETS_SYNC_INVOICES_SPREADSHEET_ID on the sheets-sync side).
func (c *Client) NextInvoiceNumber(ctx context.Context) (*NextNumberResult, error) {
	return c.nextNumber(ctx, "/next-invoice-number")
}

func (c *Client) nextNumber(ctx context.Context, path string) (*NextNumberResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("SHEETS_SYNC_URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
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

// RegisterInvoice writes a new row for a just-created invoice. Like
// RegisterAct, sheets-sync recomputes the next row/number itself at write
// time rather than trusting a caller-supplied value. Category is one of the
// three fixed project buckets ("Продвижение"/"Разработка"/"Хостинг") a
// human picks in the UI — it is NOT derived from the invoice's service line
// items, which can be several and don't map 1:1 onto a bucket. sheets-sync
// validates it server-side too, but callers should only ever pass one of
// the three values to begin with.
func (c *Client) RegisterInvoice(ctx context.Context, legalName, counterparty, category, date string) (*WriteRowResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("SHEETS_SYNC_URL is not configured")
	}
	payload, err := json.Marshal(map[string]string{
		"legal_name":   legalName,
		"counterparty": counterparty,
		"category":     category,
		"date":         date,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/write-invoice-row", bytes.NewReader(payload))
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

// GoogleStatus — состояние Google-токена, через который sheets-sync ходит в
// таблицы. OAuth-приложение в тестовом режиме, поэтому refresh-токен умирает
// примерно через 7 дней после согласия, и его приходится переподключать.
type GoogleStatus struct {
	Authenticated   bool    `json:"authenticated"`
	State           string  `json:"state"`
	Message         string  `json:"message"`
	AuthorizedAt    *string `json:"authorized_at"`
	HasClientSecret bool    `json:"has_client_secret"`
}

func (c *Client) GoogleStatus(ctx context.Context) (*GoogleStatus, error) {
	var result GoogleStatus
	if err := c.do(ctx, http.MethodGet, "/google/status", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GoogleAuthURL starts a fresh OAuth session on the sheets-sync side (it keeps
// the state/PKCE verifier) and returns the consent link for the browser.
func (c *Client) GoogleAuthURL(ctx context.Context) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	if err := c.do(ctx, http.MethodPost, "/google/auth-url", struct{}{}, &result); err != nil {
		return "", err
	}
	return result.URL, nil
}

// GoogleAuthCode exchanges the redirect URL (or bare code) the user pasted
// after consent for a new token.
func (c *Client) GoogleAuthCode(ctx context.Context, code string) (*GoogleStatus, error) {
	var result GoogleStatus
	if err := c.do(ctx, http.MethodPost, "/google/auth-code", map[string]string{"code": code}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) do(ctx context.Context, method, path string, payload, out any) error {
	if !c.Configured() {
		return fmt.Errorf("SHEETS_SYNC_URL is not configured")
	}
	var reqBody io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling sheets-sync: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return apiError(body, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing sheets-sync response: %w", err)
	}
	return nil
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
