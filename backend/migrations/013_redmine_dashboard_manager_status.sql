-- Make the Redmine dashboard match the operational manager workflow.

BEGIN;

ALTER TABLE redmine_project_dashboard_items
    ADD COLUMN IF NOT EXISTS manual_manager_id TEXT,
    ADD COLUMN IF NOT EXISTS manual_manager_name TEXT;

DO $$
DECLARE
    active_id UUID;
    new_id UUID;
BEGIN
    SELECT id INTO active_id FROM redmine_project_groups WHERE name = 'В работе' LIMIT 1;
    SELECT id INTO new_id FROM redmine_project_groups WHERE name = 'Новые' LIMIT 1;

    IF active_id IS NOT NULL THEN
        UPDATE redmine_project_groups
        SET name = 'Активные', color = '#16a34a', position = 10
        WHERE id = active_id;
    ELSE
        INSERT INTO redmine_project_groups (name, color, position)
        VALUES ('Активные', '#16a34a', 10)
        RETURNING id INTO active_id;
    END IF;

    IF new_id IS NOT NULL AND active_id IS NOT NULL THEN
        UPDATE redmine_project_dashboard_items
        SET group_id = active_id
        WHERE group_id = new_id;

        DELETE FROM redmine_project_groups WHERE id = new_id;
    END IF;
END $$;

UPDATE redmine_project_groups
SET name = 'Пауза', color = '#ca8a04', position = 20
WHERE name = 'На паузе';

UPDATE redmine_project_groups
SET name = 'Завершенные', color = '#64748b', position = 30
WHERE name = 'Архив';

INSERT INTO redmine_project_groups (name, color, position)
VALUES
    ('Активные', '#16a34a', 10),
    ('Пауза', '#ca8a04', 20),
    ('Завершенные', '#64748b', 30)
ON CONFLICT (name) DO NOTHING;

COMMIT;
