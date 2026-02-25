-- Repair contract_number source of truth
-- Ensure invoices.contract_number exists and is populated.

ALTER TABLE invoices
ADD COLUMN IF NOT EXISTS contract_number VARCHAR(100) DEFAULT 'Основной';

-- If customers.contract_number exists, use it to fill empty invoice values.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'customers' AND column_name = 'contract_number'
    ) THEN
        UPDATE invoices i
        SET contract_number = c.contract_number
        FROM customers c
        WHERE i.customer_id = c.id
          AND (i.contract_number IS NULL OR i.contract_number = '');
    END IF;
END $$;
