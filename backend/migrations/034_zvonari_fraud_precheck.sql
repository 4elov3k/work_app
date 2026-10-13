-- Звонари: фрод-предфильтр на этапе транскрибации (аудио + текст одним
-- запросом к OpenRouter, см. transcribe.Client.transcribeOpenRouter), отдельно
-- от analytics_json — то поле пишет отдельный анализ по 4 критериям
-- (см. следующий этап), не относящийся к фроду вообще. fraud_review_status
-- различает "модель считает фродом" от "человек подтвердил" — по требованию:
-- не исключать из оплаты автоматически, только флаг + ручное подтверждение.
ALTER TABLE calls ADD COLUMN IF NOT EXISTS fraud_precheck_suspected BOOLEAN;
ALTER TABLE calls ADD COLUMN IF NOT EXISTS fraud_precheck_check VARCHAR(50);
ALTER TABLE calls ADD COLUMN IF NOT EXISTS fraud_precheck_quote TEXT;
ALTER TABLE calls ADD COLUMN IF NOT EXISTS fraud_precheck_position VARCHAR(10);
ALTER TABLE calls ADD COLUMN IF NOT EXISTS fraud_review_status VARCHAR(20);
ALTER TABLE calls ADD COLUMN IF NOT EXISTS fraud_reviewed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_calls_fraud_precheck_suspected ON calls (fraud_precheck_suspected) WHERE fraud_precheck_suspected = TRUE;
CREATE INDEX IF NOT EXISTS idx_calls_fraud_review_status ON calls (fraud_review_status) WHERE fraud_review_status IS NOT NULL;
