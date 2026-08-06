// Package pbx talks to the OnlinePBX HTTP API (api2.onlinepbx.ru) to pull
// call history (CDR), the current list of internal extensions ("звонари"),
// and short-lived download links for call recordings. The auth.json flow
// exchanges the static PBX_API_TOKEN for a key_id:key session pair; that
// pair is cached in memory and refreshed once whenever a request comes
// back with OnlinePBX's WRONG_AUTH_DATA error, rather than tracked against
// an assumed TTL (OnlinePBX doesn't document one).
package pbx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	baseURL    string
	authKey    string
	httpClient *http.Client

	mu         sync.Mutex
	sessionHdr string
}

func NewFromEnv() *Client {
	domain := strings.TrimSpace(os.Getenv("PBX_DOMAIN"))
	var baseURL string
	if domain != "" {
		baseURL = "https://api2.onlinepbx.ru/" + domain
	}
	return &Client{
		baseURL:    baseURL,
		authKey:    os.Getenv("PBX_API_TOKEN"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.authKey != ""
}

// User представляет внутренний номер (звонаря) из user/get.json
type User struct {
	Num     string `json:"num"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ListUsers возвращает всех внутренних пользователей АТС
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	body, err := c.post(ctx, "user/get.json", map[string]string{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status string `json:"status"`
		Data   []User `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("onlinepbx user/get.json: parsing response: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("onlinepbx user/get.json returned status %s", parsed.Status)
	}
	return parsed.Data, nil
}

// Event — элемент events[] в CDR-записи (переводы/ответ конкретного пользователя)
type Event struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Number    string `json:"number"`
}

// CallRecord представляет одну строку CDR из mongo_history/search.json
type CallRecord struct {
	UUID              string  `json:"uuid"`
	CallerIDNumber    string  `json:"caller_id_number"`
	DestinationNumber string  `json:"destination_number"`
	StartStamp        int64   `json:"start_stamp"`
	EndStamp          int64   `json:"end_stamp"`
	Duration          int     `json:"duration"`
	UserTalkTime      int     `json:"user_talk_time"`
	HangupCause       string  `json:"hangup_cause"`
	Accountcode       string  `json:"accountcode"`
	Events            []Event `json:"events"`
}

// maxSearchWindow — жёсткий лимит OnlinePBX на один запрос mongo_history/search.json
// (кроме запросов по конкретному uuid).
const maxSearchWindow = 7 * 24 * time.Hour

// SearchHistory возвращает звонки за [from, to), разбивая запрос на
// недельные окна, и просит сервер отфильтровать короткие звонки
// (duration_from=10) — оставшийся критерий "не отменён" (hangup_cause)
// проверяется на стороне вызывающего кода, серверного фильтра для него нет.
func (c *Client) SearchHistory(ctx context.Context, from, to time.Time) ([]CallRecord, error) {
	var all []CallRecord
	windowStart := from
	for windowStart.Before(to) {
		windowEnd := windowStart.Add(maxSearchWindow)
		if windowEnd.After(to) {
			windowEnd = to
		}
		records, err := c.searchWindow(ctx, windowStart, windowEnd)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
		windowStart = windowEnd
	}
	return all, nil
}

func (c *Client) searchWindow(ctx context.Context, from, to time.Time) ([]CallRecord, error) {
	body, err := c.post(ctx, "mongo_history/search.json", map[string]interface{}{
		"start_stamp_from": from.Unix(),
		"start_stamp_to":   to.Unix(),
		"duration_from":    10,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status string       `json:"status"`
		Data   []CallRecord `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("onlinepbx mongo_history/search.json: parsing response: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("onlinepbx mongo_history/search.json returned status %s", parsed.Status)
	}
	return parsed.Data, nil
}

// DownloadRecording resolves a short-lived signed URL for the call's audio
// (valid ~30 minutes per OnlinePBX docs) and fetches it immediately.
func (c *Client) DownloadRecording(ctx context.Context, uuid string) ([]byte, error) {
	body, err := c.post(ctx, "mongo_history/search.json", map[string]interface{}{
		"uuid":     uuid,
		"download": true,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status string `json:"status"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("onlinepbx recording link: parsing response: %w", err)
	}
	if parsed.Status != "1" || parsed.Data == "" {
		return nil, fmt.Errorf("onlinepbx: no recording available for call %s", uuid)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.Data, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading recording: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading recording: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) authenticate(ctx context.Context) (string, error) {
	payload, _ := json.Marshal(map[string]string{"auth_key": c.authKey})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth.json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("onlinepbx auth: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Status string `json:"status"`
		Data   struct {
			Key   string `json:"key"`
			KeyID string `json:"key_id"`
		} `json:"data"`
		Comment string `json:"comment"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("onlinepbx auth: parsing response: %w", err)
	}
	if parsed.Status != "1" {
		return "", fmt.Errorf("onlinepbx auth failed: %s", parsed.Comment)
	}

	header := parsed.Data.KeyID + ":" + parsed.Data.Key
	c.mu.Lock()
	c.sessionHdr = header
	c.mu.Unlock()
	return header, nil
}

func (c *Client) sessionHeader(ctx context.Context) (string, error) {
	c.mu.Lock()
	cached := c.sessionHdr
	c.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	return c.authenticate(ctx)
}

// post sends a POST with the current session header, and retries once with
// a fresh session if OnlinePBX reports the session as invalid/expired.
func (c *Client) post(ctx context.Context, path string, payload interface{}) ([]byte, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("PBX_API_TOKEN/PBX_DOMAIN is not configured")
	}

	body, err := c.doPost(ctx, path, payload)
	if err != nil {
		return nil, err
	}
	if isAuthError(body) {
		if _, err := c.authenticate(ctx); err != nil {
			return nil, err
		}
		body, err = c.doPost(ctx, path, payload)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func (c *Client) doPost(ctx context.Context, path string, payload interface{}) ([]byte, error) {
	hdr, err := c.sessionHeader(ctx)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-pbx-authentication", hdr)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("onlinepbx %s: %w", path, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func isAuthError(body []byte) bool {
	var probe struct {
		ErrorCode string `json:"errorCode"`
	}
	_ = json.Unmarshal(body, &probe)
	return probe.ErrorCode == "WRONG_AUTH_DATA"
}
