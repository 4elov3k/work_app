-- Operational dashboard fields and cyclical control events for Redmine projects.

BEGIN;

ALTER TABLE redmine_project_dashboard_items
    ADD COLUMN IF NOT EXISTS manual_project_type TEXT,
    ADD COLUMN IF NOT EXISTS urgent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS urgent_reason TEXT;

CREATE TABLE IF NOT EXISTS redmine_project_control_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    redmine_project_id TEXT NOT NULL REFERENCES redmine_project_dashboard_items(redmine_project_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    service_type TEXT NOT NULL,
    title TEXT NOT NULL,
    due_date DATE NOT NULL,
    period_start DATE,
    period_end DATE,
    sequence_number INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'planned',
    sent_at TIMESTAMP WITH TIME ZONE,
    sent_by TEXT,
    redmine_issue_id TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT redmine_project_control_events_event_type_check CHECK (
        event_type IN ('report_date', 'control_cut', 'roadmap_milestone')
    ),
    CONSTRAINT redmine_project_control_events_service_type_check CHECK (
        service_type IN ('seo', 'ads', 'dev', 'legal', 'support')
    ),
    CONSTRAINT redmine_project_control_events_status_check CHECK (
        status IN ('planned', 'sent', 'skipped')
    )
);

CREATE INDEX IF NOT EXISTS idx_redmine_project_control_events_project
    ON redmine_project_control_events(redmine_project_id, status, due_date);

CREATE INDEX IF NOT EXISTS idx_redmine_project_dashboard_items_project_type
    ON redmine_project_dashboard_items(manual_project_type);

CREATE TRIGGER update_redmine_project_control_events_updated_at BEFORE UPDATE ON redmine_project_control_events
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
