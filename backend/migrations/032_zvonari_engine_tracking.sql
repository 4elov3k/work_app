-- Звонари: отслеживание, на каком движке (CPU/GPU) фактически прошла
-- транскрибация каждого звонка — нужно, чтобы подтверждение перед
-- "Перетранскрибировать на GPU" могло показать реальное число звонков,
-- которые ещё не были на GPU, а не гнать вслепую весь период (см.
-- zvonari-improvements.md, задача 4).
ALTER TABLE calls ADD COLUMN IF NOT EXISTS engine VARCHAR(10);
ALTER TABLE calls ADD COLUMN IF NOT EXISTS transcribed_at TIMESTAMP WITH TIME ZONE;
