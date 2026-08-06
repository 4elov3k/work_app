BEGIN;

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_type TEXT NOT NULL DEFAULT 'individual_entrepreneur',
    full_name TEXT NOT NULL,
    short_name TEXT NOT NULL,
    last_name TEXT,
    first_name TEXT,
    middle_name TEXT,
    inn VARCHAR(12) NOT NULL,
    kpp VARCHAR(9),
    ogrn VARCHAR(15),
    legal_address TEXT,
    postal_address TEXT,
    phone TEXT,
    email TEXT,
    bank_account TEXT,
    bank_name TEXT,
    bank_bik TEXT,
    bank_corr_account TEXT,
    tax_regime TEXT NOT NULL DEFAULT 'usn',
    vat_mode TEXT NOT NULL DEFAULT 'none',
    timezone TEXT NOT NULL DEFAULT 'Europe/Moscow',
    edo_participant_id TEXT,
    signer JSONB NOT NULL DEFAULT '{}'::jsonb,
    numbering_settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_active_one
    ON organizations(active)
    WHERE active;

INSERT INTO organizations (
    full_name,
    short_name,
    last_name,
    first_name,
    middle_name,
    inn,
    ogrn,
    legal_address,
    postal_address,
    phone,
    bank_account,
    bank_name,
    bank_bik,
    bank_corr_account,
    tax_regime,
    vat_mode,
    timezone,
    edo_participant_id,
    signer,
    numbering_settings,
    active
) VALUES (
    'Индивидуальный предприниматель Мыленкова Любовь Валерьевна',
    'ИП Мыленкова Л.В.',
    'Мыленкова',
    'Любовь',
    'Валерьевна',
    '526220116209',
    '312526227100047',
    '603136, г. Нижний Новгород ул, Маршала Рокоссовского, д. 2к1, кв 135',
    '603136, г. Нижний Новгород ул, Маршала Рокоссовского, д. 2к1, кв 135',
    '8-905-864445',
    '40802810164270001108',
    'ООО "Банк Точка"',
    '044525104',
    '30101810445745251004',
    'usn',
    'none',
    'Europe/Moscow',
    '2BEb25cae8e664f11e38742005056917125',
    '{"position":"Индивидуальный предприниматель","last_name":"Мыленкова","first_name":"Любовь","middle_name":"Валерьевна"}'::jsonb,
    '{"mode":"unified","yearly_reset":false,"invoice_start":2999,"act_start":2999,"contract_start":699}'::jsonb,
    true
)
ON CONFLICT DO NOTHING;

ALTER TABLE customers ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS archived_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS contact_person TEXT DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS contact_position TEXT DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS phone TEXT DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS email TEXT DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS comment TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_customers_status ON customers(status);
CREATE INDEX IF NOT EXISTS idx_customers_kpp ON customers(kpp);
CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);
CREATE INDEX IF NOT EXISTS idx_customers_phone ON customers(phone);

CREATE TABLE IF NOT EXISTS accounting_confirmation_tokens (
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    payload JSONB NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_accounting_confirmation_tokens_user_action
    ON accounting_confirmation_tokens(user_id, action);

CREATE TABLE IF NOT EXISTS accounting_idempotency_keys (
    action TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    result JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (action, idempotency_key)
);

CREATE TABLE IF NOT EXISTS accounting_number_sequences (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    document_type TEXT NOT NULL,
    period_year INTEGER NOT NULL DEFAULT 0,
    last_number BIGINT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_id, document_type, period_year)
);

CREATE TABLE IF NOT EXISTS document_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    document_type TEXT NOT NULL,
    document_id UUID NOT NULL,
    file_kind TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    file_name TEXT NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (document_type, document_id, file_kind)
);

CREATE INDEX IF NOT EXISTS idx_document_files_document
    ON document_files(document_type, document_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor TEXT NOT NULL,
    mcp_client TEXT,
    tool TEXT NOT NULL,
    request_id TEXT,
    document_type TEXT,
    document_id UUID,
    action TEXT NOT NULL,
    old_values JSONB,
    new_values JSONB,
    result TEXT NOT NULL,
    error_code TEXT,
    idempotency_key TEXT,
    service_ip TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_document ON audit_logs(document_type, document_id);

DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
