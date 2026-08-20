// Package notify talks to a Hermes notification endpoint that does not
// exist yet as far as this repo's code shows — this client defines the
// work_app side of that contract (payload shape, auth, endpoint path) so
// the scheduler in backend/internal/database/redmine_notify.go has
// something concrete to call. It follows the same shape as the other
// Hermes clients in this backend (see sheetsync.Client for the canonical
// example): NewFromEnv() reads a "..._URL"/"..._TOKEN" pair, Configured()
// reports whether the URL is set so callers can no-op cleanly, and the one
// method here POSTs JSON and returns a typed result or error.
//
// The env vars are prefixed REDMINE_NOTIFY_ rather than just NOTIFY_
// because this is specifically about redmine_project_control_events
// deadlines today; a future non-Redmine notification need should get its
// own client/env-var pair rather than overloading this one.
package notify

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
	baseURL := strings.TrimRight(os.Getenv("REDMINE_NOTIFY_URL"), "/")
	return &Client{
		baseURL: baseURL,
		token:   os.Getenv("REDMINE_NOTIFY_TOKEN"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != ""
}

// ControlEventDueSoonRequest is the payload for one "a control-event
// deadline is approaching" notification. EventID is the
// redmine_project_control_events.id — used by the receiving side as the
// idempotency key, so retried/duplicate deliveries for the same event
// don't need to be deduplicated by content. DaysRemaining can be negative
// (already overdue) since the scheduler's due-soon window can catch an
// event just after it tips into "burning" — see deadlineState in
// backend/internal/database/redmine_dashboard.go for the analogous ok/
// due_soon/burning/urgent classification used by the dashboard UI.
type ControlEventDueSoonRequest struct {
	EventID       string `json:"event_id"`
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	EventTitle    string `json:"event_title"`
	DueDate       string `json:"due_date"` // YYYY-MM-DD
	DaysRemaining int    `json:"days_remaining"`
}

type ControlEventDueSoonResult struct {
	Delivered bool `json:"delivered"`
}

// SendControlEventDueSoon tells Hermes that a control event's due_date is
// within the configured notification window. Callers should only mark the
// event as notified (notified_at) after this returns without error.
func (c *Client) SendControlEventDueSoon(ctx context.Context, payload ControlEventDueSoonRequest) (*ControlEventDueSoonResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("REDMINE_NOTIFY_URL is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/control-event-due-soon", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling redmine-notify: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(respBody, resp.StatusCode)
	}

	var result ControlEventDueSoonResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing redmine-notify response: %w", err)
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
		return fmt.Errorf("redmine-notify: %s", errPayload.Error)
	}
	return fmt.Errorf("redmine-notify returned HTTP %d", status)
}
