-- roadmap_milestone was reserved in the event_type check constraint but
-- never produced anywhere: dev-project cycles use event_type 'report_date'
-- with title 'Этап дорожной карты' (service_type = 'dev' is what the
-- frontend actually keys off of — see insertCycleEvents in
-- backend/internal/database/redmine_dashboard.go and eventSentActionLabel
-- in src/app/redmine/shared.ts). Drop the unused value from the constraint.

ALTER TABLE redmine_project_control_events
    DROP CONSTRAINT redmine_project_control_events_event_type_check;

ALTER TABLE redmine_project_control_events
    ADD CONSTRAINT redmine_project_control_events_event_type_check CHECK (
        event_type IN ('report_date', 'control_cut')
    );
