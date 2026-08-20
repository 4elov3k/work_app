package database

import (
	"sort"
	"testing"

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
