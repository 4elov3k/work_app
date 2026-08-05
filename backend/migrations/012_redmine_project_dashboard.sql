-- Cached Redmine project dashboard with local visual grouping.

BEGIN;

CREATE TABLE IF NOT EXISTS redmine_project_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT '#64748b',
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS redmine_project_dashboard_items (
    redmine_project_id TEXT PRIMARY KEY,
    redmine_identifier TEXT NOT NULL,
    redmine_project_name TEXT NOT NULL,
    redmine_project_url TEXT,
    description TEXT,
    status INTEGER NOT NULL DEFAULT 0,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    inferred_manager_id TEXT,
    inferred_manager_name TEXT,
    inferred_issue_id TEXT,
    inferred_at TIMESTAMP WITH TIME ZONE,
    group_id UUID REFERENCES redmine_project_groups(id) ON DELETE SET NULL,
    group_assigned_manually BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_redmine_project_dashboard_items_group_id
    ON redmine_project_dashboard_items(group_id);
CREATE INDEX IF NOT EXISTS idx_redmine_project_dashboard_items_manager
    ON redmine_project_dashboard_items(inferred_manager_name);

INSERT INTO redmine_project_groups (name, color, position)
VALUES
    ('Новые', '#2563eb', 10),
    ('В работе', '#16a34a', 20),
    ('На паузе', '#ca8a04', 30),
    ('Архив', '#64748b', 40)
ON CONFLICT (name) DO NOTHING;

CREATE TRIGGER update_redmine_project_groups_updated_at BEFORE UPDATE ON redmine_project_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_redmine_project_dashboard_items_updated_at BEFORE UPDATE ON redmine_project_dashboard_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
