-- Support a scheduled "control event due soon" notification (see
-- backend/internal/notify and backend/internal/database/redmine_notify.go):
-- notified_at records when we last told Hermes about this event's
-- upcoming due_date, so the scheduler can select rows it hasn't already
-- notified for instead of re-sending every tick.

ALTER TABLE redmine_project_control_events
    ADD COLUMN notified_at TIMESTAMPTZ;
