-- "Приложение к договору" (contract appendix / смета) — a standalone printable
-- document listing the specific work items (catalog or custom) agreed for a
-- contract, with per-section and grand totals. Mirrors invoices/acts closely
-- so it can reuse the same print/export conventions.

ALTER TABLE services ADD COLUMN IF NOT EXISTS unit VARCHAR(50) NOT NULL DEFAULT 'услуга';
ALTER TABLE services ADD COLUMN IF NOT EXISTS section VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE services ADD COLUMN IF NOT EXISTS price_per_hour DECIMAL(10, 2);
ALTER TABLE services ADD COLUMN IF NOT EXISTS hours_per_unit DECIMAL(10, 2);
ALTER TABLE services ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_services_section ON services(section);

CREATE TABLE IF NOT EXISTS contract_appendices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    number VARCHAR(50) NOT NULL,
    date VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    total_amount DECIMAL(12, 2) NOT NULL DEFAULT 0,
    archived BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT contract_appendices_unique_number UNIQUE (contract_id, number)
);
CREATE INDEX IF NOT EXISTS idx_contract_appendices_contract_id ON contract_appendices(contract_id);

CREATE TABLE IF NOT EXISTS contract_appendix_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appendix_id UUID NOT NULL REFERENCES contract_appendices(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    section VARCHAR(255) NOT NULL DEFAULT '',
    position INT NOT NULL DEFAULT 0,
    title_snapshot TEXT NOT NULL,
    unit_snapshot VARCHAR(50) NOT NULL DEFAULT 'услуга',
    price_snapshot DECIMAL(10, 2) NOT NULL DEFAULT 0,
    qty DECIMAL(10, 2) NOT NULL DEFAULT 1,
    amount DECIMAL(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_contract_appendix_lines_appendix_id ON contract_appendix_lines(appendix_id);

DROP TRIGGER IF EXISTS update_contract_appendices_updated_at ON contract_appendices;
CREATE TRIGGER update_contract_appendices_updated_at BEFORE UPDATE ON contract_appendices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
