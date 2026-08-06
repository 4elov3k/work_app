-- Звонари: звонки, синхронизированные из OnlinePBX CDR, их транскрибация
-- (Hermes, faster-whisper локально) и ChatGPT-аналитика поверх текста.
--
-- period_start/period_end в caller_reports — VARCHAR, а не DATE, тем же
-- способом, что и остальные "даты" в этой схеме (см. invoices.date,
-- contract_appendices.date) — драйвер lib/pq иначе декодирует DATE в
-- time.Time, а не в string, которым тут представлены поля моделей.

CREATE TABLE IF NOT EXISTS callers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pbx_extension VARCHAR(20) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_callers_pbx_extension ON callers(pbx_extension);

CREATE TABLE IF NOT EXISTS calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pbx_uuid VARCHAR(64) NOT NULL UNIQUE,
    caller_id UUID REFERENCES callers(id) ON DELETE SET NULL,
    direction VARCHAR(20) NOT NULL,
    counterparty_number VARCHAR(32) NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_sec INTEGER NOT NULL DEFAULT 0,
    talk_time_sec INTEGER NOT NULL DEFAULT 0,
    hangup_cause VARCHAR(50) NOT NULL DEFAULT '',
    -- pending -> transcribing -> done, или no_recording / failed
    transcript_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    transcript_text TEXT,
    analytics_json JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_calls_caller_id ON calls(caller_id);
CREATE INDEX idx_calls_started_at ON calls(started_at);
CREATE INDEX idx_calls_transcript_status ON calls(transcript_status);

CREATE TABLE IF NOT EXISTS caller_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    caller_id UUID NOT NULL REFERENCES callers(id) ON DELETE CASCADE,
    period VARCHAR(10) NOT NULL,
    period_start VARCHAR(10) NOT NULL,
    period_end VARCHAR(10) NOT NULL,
    summary_text TEXT NOT NULL DEFAULT '',
    metrics_json JSONB,
    requested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_caller_reports_caller_id ON caller_reports(caller_id);

CREATE TRIGGER update_callers_updated_at BEFORE UPDATE ON callers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_calls_updated_at BEFORE UPDATE ON calls
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
