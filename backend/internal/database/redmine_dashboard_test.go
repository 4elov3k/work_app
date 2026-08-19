package database

import (
	"testing"
	"time"

	"invoices-backend/internal/models"
)

// referenceNow anchors "today" for deadlineState tests. Using a time with a
// non-zero hour/minute pins down that deadlineState compares calendar days,
// not raw 24h durations, since it truncates now to midnight before diffing.
var referenceNow = time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC)

func dueDateEvent(dueDate string) *models.RedmineProjectControlEvent {
	return &models.RedmineProjectControlEvent{DueDate: dueDate}
}

func TestDeadlineState_Urgent(t *testing.T) {
	// urgent=true always wins, regardless of event/due date.
	tests := []struct {
		name  string
		event *models.RedmineProjectControlEvent
	}{
		{"nil event", nil},
		{"no due date", dueDateEvent("")},
		{"due date far in the future", dueDateEvent("2030-01-01")},
		{"due date in the past", dueDateEvent("2020-01-01")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deadlineState(true, tt.event, referenceNow); got != "urgent" {
				t.Errorf("deadlineState(true, %v, now) = %q, want urgent", tt.event, got)
			}
		})
	}
}

func TestDeadlineState_NoEventOrNoDueDate(t *testing.T) {
	if got := deadlineState(false, nil, referenceNow); got != "ok" {
		t.Errorf("deadlineState(false, nil, now) = %q, want ok", got)
	}
	if got := deadlineState(false, dueDateEvent(""), referenceNow); got != "ok" {
		t.Errorf("deadlineState(false, empty due date, now) = %q, want ok", got)
	}
}

func TestDeadlineState_UnparseableDueDate(t *testing.T) {
	if got := deadlineState(false, dueDateEvent("not-a-date"), referenceNow); got != "ok" {
		t.Errorf("deadlineState(false, unparseable due date, now) = %q, want ok", got)
	}
}

func TestDeadlineState_Boundaries(t *testing.T) {
	tests := []struct {
		name    string
		dueDate string
		want    string
	}{
		{"1 day overdue", "2026-08-19", "burning"},
		{"due today (0 days)", "2026-08-20", "due_soon"},
		{"3 days out (still due_soon)", "2026-08-23", "due_soon"},
		{"4 days out (first ok day)", "2026-08-24", "ok"},
		{"far in the future", "2027-01-01", "ok"},
		{"far overdue", "2020-01-01", "burning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deadlineState(false, dueDateEvent(tt.dueDate), referenceNow)
			if got != tt.want {
				t.Errorf("deadlineState(false, due=%s, now=%s) = %q, want %q", tt.dueDate, referenceNow, got, tt.want)
			}
		})
	}
}

func TestDateOnly(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	in := time.Date(2026, 3, 15, 23, 59, 59, 999999999, loc)
	got := dateOnly(in)

	want := time.Date(2026, 3, 15, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("dateOnly(%v) = %v, want %v", in, got, want)
	}
	if got.Location() != loc {
		t.Errorf("dateOnly(%v) location = %v, want %v (location must be preserved)", in, got.Location(), loc)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
		t.Errorf("dateOnly(%v) = %v, want time-of-day truncated to midnight", in, got)
	}
}

func TestDateOnly_AlreadyMidnight(t *testing.T) {
	in := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := dateOnly(in)
	if !got.Equal(in) {
		t.Errorf("dateOnly(%v) = %v, want unchanged %v", in, got, in)
	}
}
