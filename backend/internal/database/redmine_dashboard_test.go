package database

import (
	"sort"
	"testing"
	"time"

	"invoices-backend/internal/models"
)

// TestCollectManagerOptionsDedupesByIDNotName is the regression test for the
// "Разграничение менеджеров с одинаковыми именами" bug: two dashboard items
// whose effective manager has the same display name but a different manager
// ID must produce two distinct entries in the managers list, not one entry
// that silently picks whichever manager happened to win a map iteration.
func TestCollectManagerOptionsDedupesByIDNotName(t *testing.T) {
	items := []models.RedmineProjectDashboardItem{
		{
			ProjectID:            "p1",
			EffectiveManagerID:   "user-1",
			EffectiveManagerName: "Иван Иванов",
		},
		{
			ProjectID:            "p2",
			EffectiveManagerID:   "user-2",
			EffectiveManagerName: "Иван Иванов",
		},
	}

	managers := collectManagerOptions(items)

	if len(managers) != 2 {
		t.Fatalf("expected 2 distinct managers for same name/different IDs, got %d: %+v", len(managers), managers)
	}

	ids := map[string]bool{}
	for _, m := range managers {
		if m.Name != "Иван Иванов" {
			t.Fatalf("unexpected manager name: %+v", m)
		}
		ids[m.ID] = true
	}
	if !ids["user-1"] || !ids["user-2"] {
		t.Fatalf("expected both user-1 and user-2 to be present, got %+v", managers)
	}
}

func TestCollectManagerOptionsDedupesByID(t *testing.T) {
	items := []models.RedmineProjectDashboardItem{
		{
			ProjectID:            "p1",
			EffectiveManagerID:   "user-1",
			EffectiveManagerName: "Иван Иванов",
		},
		{
			ProjectID:            "p2",
			EffectiveManagerID:   "user-1",
			EffectiveManagerName: "Иван Иванов",
			InferredManagerID:    "user-1",
			InferredManagerName:  "Иван Иванов",
		},
	}

	managers := collectManagerOptions(items)

	if len(managers) != 1 {
		t.Fatalf("expected the same manager ID to be deduplicated to 1 entry, got %d: %+v", len(managers), managers)
	}
	if managers[0].ID != "user-1" || managers[0].Name != "Иван Иванов" {
		t.Fatalf("unexpected manager entry: %+v", managers[0])
	}
}

func TestCollectManagerOptionsFallsBackToNameWhenIDMissing(t *testing.T) {
	items := []models.RedmineProjectDashboardItem{
		{
			ProjectID:            "p1",
			EffectiveManagerID:   "",
			EffectiveManagerName: "Легаси Менеджер",
		},
		{
			ProjectID:            "p2",
			EffectiveManagerID:   "",
			EffectiveManagerName: "Легаси Менеджер",
		},
	}

	managers := collectManagerOptions(items)

	if len(managers) != 1 {
		t.Fatalf("expected id-less entries with the same name to be deduplicated by name, got %d: %+v", len(managers), managers)
	}
	if managers[0].ID != "" || managers[0].Name != "Легаси Менеджер" {
		t.Fatalf("unexpected manager entry: %+v", managers[0])
	}
}

func TestCollectManagerOptionsSortedByName(t *testing.T) {
	items := []models.RedmineProjectDashboardItem{
		{ProjectID: "p1", EffectiveManagerID: "user-2", EffectiveManagerName: "Борис"},
		{ProjectID: "p2", EffectiveManagerID: "user-1", EffectiveManagerName: "Анна"},
	}

	managers := collectManagerOptions(items)

	if !sort.SliceIsSorted(managers, func(i, j int) bool { return managers[i].Name < managers[j].Name }) {
		t.Fatalf("expected managers sorted by name, got %+v", managers)
	}
}

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
