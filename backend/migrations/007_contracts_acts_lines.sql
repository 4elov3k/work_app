-- Contracts + Acts + Line items + Integrity constraints

BEGIN;

-- Contracts (root entity)
CREATE TABLE IF NOT EXISTS contracts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    number VARCHAR(100) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    status TEXT NOT NULL DEFAULT 'active',
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT contracts_status_check CHECK (status IN ('draft','active','closed','canceled')),
    CONSTRAINT contracts_unique_number UNIQUE (customer_id, number)
);

CREATE INDEX IF NOT EXISTS idx_contracts_customer_id ON contracts(customer_id);

-- Invoices: add contract linkage and status/total
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS contract_id UUID;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS contract_number VARCHAR(100) DEFAULT 'Основной';
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'draft';
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS total_amount NUMERIC(12, 2) DEFAULT 0;

-- Customers: ensure contract_number exists for migration compatibility
ALTER TABLE customers ADD COLUMN IF NOT EXISTS contract_number VARCHAR(100) DEFAULT 'Основной';
CREATE INDEX IF NOT EXISTS idx_customers_contract_number ON customers(contract_number);

-- Acts
CREATE TABLE IF NOT EXISTS acts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    number VARCHAR(50) NOT NULL,
    date VARCHAR(20) NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    total_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT acts_status_check CHECK (status IN ('draft','signed','canceled')),
    CONSTRAINT acts_unique_number UNIQUE (contract_id, number)
);

CREATE INDEX IF NOT EXISTS idx_acts_contract_id ON acts(contract_id);

-- Invoice lines with snapshots
CREATE TABLE IF NOT EXISTS invoice_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    title_snapshot TEXT NOT NULL,
    unit_snapshot TEXT NOT NULL DEFAULT 'шт',
    vat_snapshot NUMERIC(5, 2) NOT NULL DEFAULT 0,
    price_snapshot NUMERIC(12, 2) NOT NULL,
    qty NUMERIC(12, 3) NOT NULL,
    amount NUMERIC(12, 2) NOT NULL,
    CONSTRAINT invoice_lines_qty_check CHECK (qty > 0),
    CONSTRAINT invoice_lines_price_check CHECK (price_snapshot >= 0),
    CONSTRAINT invoice_lines_amount_check CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_invoice_lines_invoice_id ON invoice_lines(invoice_id);
CREATE INDEX IF NOT EXISTS idx_invoice_lines_service_id ON invoice_lines(service_id);

-- Act lines with snapshots
CREATE TABLE IF NOT EXISTS act_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    title_snapshot TEXT NOT NULL,
    unit_snapshot TEXT NOT NULL DEFAULT 'шт',
    vat_snapshot NUMERIC(5, 2) NOT NULL DEFAULT 0,
    price_snapshot NUMERIC(12, 2) NOT NULL,
    qty NUMERIC(12, 3) NOT NULL,
    amount NUMERIC(12, 2) NOT NULL,
    CONSTRAINT act_lines_qty_check CHECK (qty > 0),
    CONSTRAINT act_lines_price_check CHECK (price_snapshot >= 0),
    CONSTRAINT act_lines_amount_check CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_act_lines_act_id ON act_lines(act_id);
CREATE INDEX IF NOT EXISTS idx_act_lines_service_id ON act_lines(service_id);

-- Link table: acts <-> invoices
CREATE TABLE IF NOT EXISTS act_invoices (
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE RESTRICT,
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE RESTRICT,
    PRIMARY KEY (act_id, invoice_id)
);

CREATE INDEX IF NOT EXISTS idx_act_invoices_act_id ON act_invoices(act_id);
CREATE INDEX IF NOT EXISTS idx_act_invoices_invoice_id ON act_invoices(invoice_id);

-- Backfill contracts from customers (default contract) and invoices
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'customers' AND column_name = 'contract_number'
    ) THEN
        INSERT INTO contracts (customer_id, number, currency, status, start_date)
        SELECT c.id, COALESCE(c.contract_number, 'Основной'), 'RUB', 'active', CURRENT_DATE
        FROM customers c
        ON CONFLICT (customer_id, number) DO NOTHING;
    ELSE
        INSERT INTO contracts (customer_id, number, currency, status, start_date)
        SELECT c.id, 'Основной', 'RUB', 'active', CURRENT_DATE
        FROM customers c
        ON CONFLICT (customer_id, number) DO NOTHING;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'invoices' AND column_name = 'contract_number'
    ) THEN
        INSERT INTO contracts (customer_id, number, currency, status, start_date)
        SELECT DISTINCT i.customer_id, COALESCE(i.contract_number, 'Основной'), 'RUB', 'active', CURRENT_DATE
        FROM invoices i
        ON CONFLICT (customer_id, number) DO NOTHING;
    END IF;
END $$;

-- Set contract_id on invoices
UPDATE invoices i
SET contract_id = c.id
FROM contracts c
WHERE c.customer_id = i.customer_id
  AND c.number = COALESCE(i.contract_number, 'Основной')
  AND i.contract_id IS NULL;

-- Create acts from invoices with document_type='certificate' (if column exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'invoices' AND column_name = 'document_type'
    ) THEN
        INSERT INTO acts (id, contract_id, number, date, status, total_amount, created_at, updated_at)
        SELECT i.id, i.contract_id, i.number, i.date, 'draft', 0, i.created_at, i.updated_at
        FROM invoices i
        WHERE i.document_type = 'certificate'
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- Migrate invoice_services into invoice_lines / act_lines (if table exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'invoice_services'
    ) THEN
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_name = 'invoices' AND column_name = 'document_type'
        ) THEN
            INSERT INTO invoice_lines (invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
            SELECT isv.invoice_id, s.id, s.name, 'шт', 0, s.price, 1, s.price
            FROM invoice_services isv
            JOIN invoices i ON i.id = isv.invoice_id
            JOIN services s ON s.id = isv.service_id
            WHERE i.document_type != 'certificate';

            INSERT INTO act_lines (act_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
            SELECT isv.invoice_id, s.id, s.name, 'шт', 0, s.price, 1, s.price
            FROM invoice_services isv
            JOIN invoices i ON i.id = isv.invoice_id
            JOIN services s ON s.id = isv.service_id
            WHERE i.document_type = 'certificate';
        ELSE
            INSERT INTO invoice_lines (invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
            SELECT isv.invoice_id, s.id, s.name, 'шт', 0, s.price, 1, s.price
            FROM invoice_services isv
            JOIN invoices i ON i.id = isv.invoice_id
            JOIN services s ON s.id = isv.service_id;
        END IF;
    END IF;
END $$;

-- Update totals
UPDATE invoices i
SET total_amount = sub.total
FROM (
    SELECT invoice_id, COALESCE(SUM(amount), 0) AS total
    FROM invoice_lines
    GROUP BY invoice_id
) sub
WHERE i.id = sub.invoice_id;

UPDATE acts a
SET total_amount = sub.total
FROM (
    SELECT act_id, COALESCE(SUM(amount), 0) AS total
    FROM act_lines
    GROUP BY act_id
) sub
WHERE a.id = sub.act_id;

-- Remove certificate rows from invoices (if column exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'invoices' AND column_name = 'document_type'
    ) THEN
        DELETE FROM invoices WHERE document_type = 'certificate';
    END IF;
END $$;

-- Drop old join table
DROP TABLE IF EXISTS invoice_services;

-- Drop old document_type column/type if present
ALTER TABLE invoices DROP COLUMN IF EXISTS document_type;
DROP TYPE IF EXISTS document_type;

-- Enforce contract linkage
ALTER TABLE invoices
    ALTER COLUMN contract_id SET NOT NULL;

-- Recreate customer FK with RESTRICT
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
    FROM pg_constraint
    WHERE conrelid = 'invoices'::regclass
      AND contype = 'f'
      AND confrelid = 'customers'::regclass;
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE invoices DROP CONSTRAINT %I', cname);
    END IF;
END $$;

ALTER TABLE invoices
    ADD CONSTRAINT fk_invoices_customer FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT;

ALTER TABLE invoices
    ADD CONSTRAINT fk_invoices_contract FOREIGN KEY (contract_id) REFERENCES contracts(id) ON DELETE RESTRICT;

ALTER TABLE invoices
    ADD CONSTRAINT invoices_status_check CHECK (status IN ('draft','issued','paid','canceled'));

ALTER TABLE invoices
    ADD CONSTRAINT invoices_total_check CHECK (total_amount >= 0);

ALTER TABLE invoices
    ADD CONSTRAINT invoices_unique_number UNIQUE (contract_id, number);

CREATE INDEX IF NOT EXISTS idx_invoices_contract_id ON invoices(contract_id);

-- Triggers for updated_at on new tables
CREATE TRIGGER update_contracts_updated_at BEFORE UPDATE ON contracts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_acts_updated_at BEFORE UPDATE ON acts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Enforce invoice.customer_id matches contract.customer_id
CREATE OR REPLACE FUNCTION enforce_invoice_contract_customer()
RETURNS TRIGGER AS $$
DECLARE
    contract_customer_id UUID;
BEGIN
    SELECT customer_id INTO contract_customer_id FROM contracts WHERE id = NEW.contract_id;
    IF contract_customer_id IS NULL THEN
        RAISE EXCEPTION 'Contract not found for invoice';
    END IF;
    IF NEW.customer_id <> contract_customer_id THEN
        RAISE EXCEPTION 'Invoice customer_id must match contract.customer_id';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_invoice_contract_customer ON invoices;
CREATE TRIGGER trg_invoice_contract_customer
BEFORE INSERT OR UPDATE ON invoices
FOR EACH ROW EXECUTE FUNCTION enforce_invoice_contract_customer();

-- Enforce act-invoice contract match
CREATE OR REPLACE FUNCTION enforce_act_invoice_contract()
RETURNS TRIGGER AS $$
DECLARE
    act_contract UUID;
    invoice_contract UUID;
BEGIN
    SELECT contract_id INTO act_contract FROM acts WHERE id = NEW.act_id;
    SELECT contract_id INTO invoice_contract FROM invoices WHERE id = NEW.invoice_id;
    IF act_contract IS NULL OR invoice_contract IS NULL THEN
        RAISE EXCEPTION 'Act or Invoice not found for link';
    END IF;
    IF act_contract <> invoice_contract THEN
        RAISE EXCEPTION 'Act and Invoice must belong to the same contract';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_act_invoices_contract ON act_invoices;
CREATE TRIGGER trg_act_invoices_contract
BEFORE INSERT OR UPDATE ON act_invoices
FOR EACH ROW EXECUTE FUNCTION enforce_act_invoice_contract();

COMMIT;
