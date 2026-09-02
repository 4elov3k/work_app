-- Звонари: причина ошибки транскрибации как отдельное поле, вместо того
-- чтобы "Повторить неудачные" сваливало failed/no_recording/pending в одну
-- кучу — no_recording ретраем не лечится (записи просто нет), см.
-- zvonari-improvements.md, задача 6.
ALTER TABLE calls ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE calls ADD COLUMN IF NOT EXISTS error_kind VARCHAR(30);
CREATE INDEX IF NOT EXISTS idx_calls_error_kind ON calls (error_kind) WHERE error_kind IS NOT NULL;
