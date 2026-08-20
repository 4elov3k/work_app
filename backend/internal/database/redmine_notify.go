package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ControlEventNotificationCandidate is one redmine_project_control_events
// row as seen by the "due soon" notification scheduler. It carries the
// project name alongside the event so the notify client's payload (see
// notify.ControlEventDueSoonRequest) can include something human-readable
// without a second lookup.
type ControlEventNotificationCandidate struct {
	EventID     string
	ProjectID   string
	ProjectName string
	EventTitle  string
	DueDate     string // YYYY-MM-DD
	Status      string
	NotifiedAt  *time.Time

	// DaysRemaining is filled in by selectDueSoonControlEvents (calendar
	// days between "today" and DueDate, negative once overdue) — callers
	// building a notify.ControlEventDueSoonRequest read it from here rather
	// than recomputing it, so the day-count math lives in one place.
	DaysRemaining int
}

// selectDueSoonControlEvents is the pure "which events need notifying right
// now" filter, kept separate from DB I/O and the HTTP call the same way
// deadlineState (backend/internal/database/redmine_dashboard.go) is a pure
// function separate from the dashboard query that calls it — that's what
// lets both be unit tested without a live Postgres or a live Hermes.
//
// A candidate is selected when all of:
//   - status is "planned" (sent/skipped events never get notified)
//   - notified_at is unset (a previous run already notified — don't resend)
//   - due_date is on or before now + daysBefore
//
// There is deliberately no lower bound on how overdue an event can be: a
// "planned" event whose due_date has already passed still has an unset
// notified_at, and skipping it here would silently mean it never gets
// flagged at all (nothing else in this backend proactively surfaces it —
// see deadlineState's "burning" state, which only fires when someone has
// the dashboard open). Judgment call: if this turns out to be too noisy for
// long-overdue events, add an upper bound on lateness later.
func selectDueSoonControlEvents(candidates []ControlEventNotificationCandidate, now time.Time, daysBefore int) []ControlEventNotificationCandidate {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	due := make([]ControlEventNotificationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Status != "planned" {
			continue
		}
		if candidate.NotifiedAt != nil {
			continue
		}
		dueDate, err := time.ParseInLocation("2006-01-02", candidate.DueDate, now.Location())
		if err != nil {
			continue
		}
		days := int(dueDate.Sub(today).Hours() / 24)
		if days > daysBefore {
			continue
		}
		due = append(due, candidate)
	}
	return due
}

// daysRemaining is the same day-count deadlineState uses (calendar days
// between "today" and the event's due_date, negative once overdue) — used
// to fill notify.ControlEventDueSoonRequest.DaysRemaining.
func daysRemaining(dueDate string, now time.Time) int {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	parsed, err := time.ParseInLocation("2006-01-02", dueDate, now.Location())
	if err != nil {
		return 0
	}
	return int(parsed.Sub(today).Hours() / 24)
}

// GetControlEventsDueForNotification fetches candidate rows for the "due
// soon" scheduler: planned events that have not yet been notified. The SQL
// pre-filters on status/notified_at as a scale optimization (so a growing
// history of sent/skipped events never has to be pulled back for every
// tick); selectDueSoonControlEvents re-checks those same conditions plus
// the due-date window, so the filtering logic itself lives in one pure,
// tested place rather than being split across SQL and Go.
func (db *DB) GetControlEventsDueForNotification(ctx context.Context, now time.Time, daysBefore int) ([]ControlEventNotificationCandidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT e.id::text, e.redmine_project_id, COALESCE(p.redmine_project_name, ''),
		       e.title, e.due_date, e.status, e.notified_at
		FROM redmine_project_control_events e
		LEFT JOIN redmine_project_dashboard_items p ON p.redmine_project_id = e.redmine_project_id
		WHERE e.status = 'planned' AND e.notified_at IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query control events pending notification: %w", err)
	}
	defer rows.Close()

	candidates := []ControlEventNotificationCandidate{}
	for rows.Next() {
		var candidate ControlEventNotificationCandidate
		var dueDate time.Time
		var notifiedAt sql.NullTime
		if err := rows.Scan(
			&candidate.EventID,
			&candidate.ProjectID,
			&candidate.ProjectName,
			&candidate.EventTitle,
			&dueDate,
			&candidate.Status,
			&notifiedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan control event pending notification: %w", err)
		}
		candidate.DueDate = dueDate.Format("2006-01-02")
		if notifiedAt.Valid {
			candidate.NotifiedAt = &notifiedAt.Time
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate control events pending notification: %w", err)
	}

	return selectDueSoonControlEvents(candidates, now, daysBefore), nil
}

// MarkControlEventNotified records that a "due soon" notification was
// successfully delivered for this event, so the next scheduler tick won't
// pick it up again via GetControlEventsDueForNotification.
func (db *DB) MarkControlEventNotified(ctx context.Context, eventID string) error {
	if _, err := db.ExecContext(ctx, `
		UPDATE redmine_project_control_events
		SET notified_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, eventID); err != nil {
		return fmt.Errorf("failed to mark control event notified: %w", err)
	}
	return nil
}
