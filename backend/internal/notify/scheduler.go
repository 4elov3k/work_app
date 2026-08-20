package notify

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"invoices-backend/internal/database"
)

// defaultDaysBefore matches the "due_soon" threshold used by deadlineState
// in backend/internal/database/redmine_dashboard.go, so the background
// notification and the dashboard's own due-soon badge agree by default.
const defaultDaysBefore = 3

// RunControlEventDueSoonScheduler polls redmine_project_control_events on
// an interval and asks Hermes to notify about any "planned" event whose
// due_date falls within REDMINE_NOTIFY_DAYS_BEFORE days and hasn't been
// notified yet (see database.GetControlEventsDueForNotification /
// selectDueSoonControlEvents for the selection logic).
//
// It is a silent no-op when the client isn't configured (REDMINE_NOTIFY_URL
// unset) — that's the default for every dev/CI environment today, and
// nothing here should require Hermes to be reachable just to run the
// backend. Call this in a goroutine; it blocks until ctx is done.
func RunControlEventDueSoonScheduler(ctx context.Context, db *database.DB, client *Client, interval time.Duration) {
	if !client.Configured() {
		return
	}

	daysBefore := defaultDaysBefore
	if raw := os.Getenv("REDMINE_NOTIFY_DAYS_BEFORE"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			daysBefore = parsed
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runOnce(ctx, db, client, daysBefore)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce(ctx, db, client, daysBefore)
		}
	}
}

func runOnce(ctx context.Context, db *database.DB, client *Client, daysBefore int) {
	candidates, err := db.GetControlEventsDueForNotification(ctx, time.Now(), daysBefore)
	if err != nil {
		log.Printf("redmine notify scheduler: failed to load due-soon control events: %v", err)
		return
	}
	for _, candidate := range candidates {
		_, err := client.SendControlEventDueSoon(ctx, ControlEventDueSoonRequest{
			EventID:       candidate.EventID,
			ProjectID:     candidate.ProjectID,
			ProjectName:   candidate.ProjectName,
			EventTitle:    candidate.EventTitle,
			DueDate:       candidate.DueDate,
			DaysRemaining: candidate.DaysRemaining,
		})
		if err != nil {
			log.Printf("redmine notify scheduler: failed to notify for event %s: %v", candidate.EventID, err)
			continue
		}
		if err := db.MarkControlEventNotified(ctx, candidate.EventID); err != nil {
			log.Printf("redmine notify scheduler: notified event %s but failed to record notified_at: %v", candidate.EventID, err)
		}
	}
}
