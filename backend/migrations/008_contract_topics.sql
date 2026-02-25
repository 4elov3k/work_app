BEGIN;

-- Add topic to contracts
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS topic TEXT;
UPDATE contracts SET topic = 'Продвижение сео' WHERE topic IS NULL;
ALTER TABLE contracts ALTER COLUMN topic SET NOT NULL;

-- Ensure status is set
UPDATE contracts SET status = 'active' WHERE status IS NULL OR status = '';

-- Refresh contracts status constraint to include archived
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
    FROM pg_constraint
    WHERE conrelid = 'contracts'::regclass
      AND contype = 'c'
      AND conname = 'contracts_status_check';
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE contracts DROP CONSTRAINT %I', cname);
    END IF;
END $$;

ALTER TABLE contracts
    ADD CONSTRAINT contracts_status_check CHECK (status IN ('draft','active','archived','closed','canceled'));

-- Refresh contracts topic constraint
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
    FROM pg_constraint
    WHERE conrelid = 'contracts'::regclass
      AND contype = 'c'
      AND conname = 'contracts_topic_check';
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE contracts DROP CONSTRAINT %I', cname);
    END IF;
END $$;

ALTER TABLE contracts
    ADD CONSTRAINT contracts_topic_check CHECK (
        topic IN (
            'Продвижение сео',
            'Продвижение контекст',
            'Сео + контекст',
            'Техподдержка',
            'Юр услуги',
            'Разработка',
            'Соц сети',
            'Дизайн',
            'Отзывы'
        )
    );

COMMIT;
