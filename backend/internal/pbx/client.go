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
	"errors"
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
		Status  string `json:"status"`
		Data    []User `json:"data"`
		Comment string `json:"comment"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("onlinepbx user/get.json: parsing response: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("onlinepbx user/get.json returned status %s: %s", parsed.Status, parsed.Comment)
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
// недельные окна, и просит сервер отфильтровать по времени именно
// разговора (user_talk_time_from=10), а не по общей длительности звонка —
// duration_sec включает время дозвона/гудков, так что звонок с нулевым
// talk_time (никто не ответил) мог проходить фильтр по одной "duration".
// Оставшийся критерий "не отменён" (hangup_cause) проверяется на стороне
// вызывающего кода, серверного фильтра для него нет.
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
		"start_stamp_from":    from.Unix(),
		"start_stamp_to":      to.Unix(),
		"user_talk_time_from": 10,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status  string       `json:"status"`
		Data    []CallRecord `json:"data"`
		Comment string       `json:"comment"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("onlinepbx mongo_history/search.json: parsing response: %w", err)
	}
	if parsed.Status != "1" {
		return nil, fmt.Errorf("onlinepbx mongo_history/search.json returned status %s: %s", parsed.Status, parsed.Comment)
	}
	return parsed.Data, nil
}

// ErrNoRecording is returned only when OnlinePBX explicitly confirms there
// is no recording for a call (a successful API response with an empty
// link) — as opposed to any other failure (network error, rate limiting,
// a bad HTTP status downloading the file), which is a transient/technical
// problem the caller should treat as retryable, not "there was never a
// recording". Conflating the two previously caused real recordings to be
// permanently marked "no_recording" whenever the fetch merely failed once.
var ErrNoRecording = errors.New("onlinepbx: no recording available for this call")

// RecordingURL resolves a short-lived signed URL for the call's audio
// (valid ~30 minutes per OnlinePBX docs) without fetching it. Callers that
// want a link to hand a human (e.g. a redirect endpoint) should call this
// fresh each time rather than caching the result.
func (c *Client) RecordingURL(ctx context.Context, uuid string) (string, error) {
	body, err := c.post(ctx, "mongo_history/search.json", map[string]interface{}{
		"uuid":     uuid,
		"download": true,
	})
	if err != nil {
		return "", fmt.Errorf("requesting recording link: %w", err)
	}
	var parsed struct {
		Status  string `json:"status"`
		Data    string `json:"data"`
		Comment string `json:"comment"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("onlinepbx recording link: parsing response: %w", err)
	}
	if parsed.Status != "1" {
		return "", fmt.Errorf("onlinepbx recording link: request returned status %s: %s", parsed.Status, parsed.Comment)
	}
	if parsed.Data == "" {
		return "", ErrNoRecording
	}
	return parsed.Data, nil
}

// DownloadRecording resolves a short-lived signed URL for the call's audio
// and fetches it immediately.
func (c *Client) DownloadRecording(ctx context.Context, uuid string) ([]byte, error) {
	link, err := c.RecordingURL(ctx, uuid)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
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

// isAuthError reports whether body is OnlinePBX's shape for "this session is
// no longer valid, get a fresh one" — checked via the API's own dedicated
// `isNotAuth` flag rather than matching specific errorCode strings, which
// vary by failure reason (observed both "WRONG_AUTH_DATA" for a malformed
// key and "API_KEY_CHECK_FAILED" for an expired-but-well-formed cached
// session — errorCode alone previously only caught the former, so a stale
// cached session in a long-running process never triggered the retry below
// and every request failed with a bare "status 0" until the backend was
// restarted).
func isAuthError(body []byte) bool {
	var probe struct {
		IsNotAuth bool   `json:"isNotAuth"`
		ErrorCode string `json:"errorCode"`
	}
	_ = json.Unmarshal(body, &probe)
	return probe.IsNotAuth || probe.ErrorCode == "WRONG_AUTH_DATA"
}
