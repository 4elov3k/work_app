package database

import (
	"testing"
	"time"
)

func TestSelectDueSoonControlEvents(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	const daysBefore = 3

	notifiedAt := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		candidate ControlEventNotificationCandidate
		wantMatch bool
	}{
		{
			name: "outside the N-day window is not selected",
			candidate: ControlEventNotificationCandidate{
				EventID:  "outside-window",
				DueDate:  "2026-09-01", // far more than 3 days out
				Status:   "planned",
				NotifiedAt: nil,
			},
			wantMatch: false,
		},
		{
			name: "inside window with notified_at unset is selected",
			candidate: ControlEventNotificationCandidate{
				EventID:    "inside-window-unnotified",
				DueDate:    "2026-08-22", // 2 days out, within window
				Status:     "planned",
				NotifiedAt: nil,
			},
			wantMatch: true,
		},
		{
			name: "inside window but already notified is not resent",
			candidate: ControlEventNotificationCandidate{
				EventID:    "inside-window-notified",
				DueDate:    "2026-08-22",
				Status:     "planned",
				NotifiedAt: &notifiedAt,
			},
			wantMatch: false,
		},
		{
			name: "due soon but status is not planned is not selected",
			candidate: ControlEventNotificationCandidate{
				EventID:    "sent-status",
				DueDate:    "2026-08-22",
				Status:     "sent",
				NotifiedAt: nil,
			},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectDueSoonControlEvents([]ControlEventNotificationCandidate{tt.candidate}, now, daysBefore)
			matched := len(got) == 1
			if matched != tt.wantMatch {
				t.Errorf("selectDueSoonControlEvents(%+v) matched = %v, want %v", tt.candidate, matched, tt.wantMatch)
			}
		})
	}
}

func TestSelectDueSoonControlEvents_MixedBatch(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	notifiedAt := now

	candidates := []ControlEventNotificationCandidate{
		{EventID: "keep-1", DueDate: "2026-08-20", Status: "planned"},              // due today
		{EventID: "keep-2", DueDate: "2026-08-15", Status: "planned"},              // overdue, unnotified
		{EventID: "drop-far", DueDate: "2026-12-01", Status: "planned"},            // far out
		{EventID: "drop-notified", DueDate: "2026-08-21", Status: "planned", NotifiedAt: &notifiedAt},
		{EventID: "drop-sent", DueDate: "2026-08-21", Status: "sent"},
		{EventID: "drop-skipped", DueDate: "2026-08-21", Status: "skipped"},
	}

	got := selectDueSoonControlEvents(candidates, now, 3)

	gotIDs := make(map[string]bool, len(got))
	for _, c := range got {
		gotIDs[c.EventID] = true
	}

	wantSelected := []string{"keep-1", "keep-2"}
	for _, id := range wantSelected {
		if !gotIDs[id] {
			t.Errorf("expected %q to be selected, was not", id)
		}
	}
	if len(got) != len(wantSelected) {
		t.Errorf("selectDueSoonControlEvents returned %d events, want %d (%v)", len(got), len(wantSelected), got)
	}
}

func TestDaysRemaining(t *testing.T) {
	now := time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC)

	cases := []struct {
		dueDate string
		want    int
	}{
		{"2026-08-20", 0},
		{"2026-08-23", 3},
		{"2026-08-18", -2},
	}
	for _, c := range cases {
		if got := daysRemaining(c.dueDate, now); got != c.want {
			t.Errorf("daysRemaining(%q) = %d, want %d", c.dueDate, got, c.want)
		}
	}
}
