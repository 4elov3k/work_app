-- Act/invoice numbers were only unique per contract (contract_id, number),
-- so a fresh contract's "next number" query always started counting from
-- 3000 again — different contracts (even different customers) could end up
-- with the same act or invoice number, which isn't valid for real
-- accounting/document-numbering purposes. The MCP tooling already computed
-- the next number globally (no contract_id filter); the REST path did not,
-- which is exactly the inconsistency that produced two acts numbered 3000.
--
-- Sanity check before tightening the constraint: fails loudly if duplicates
-- exist rather than silently corrupting data.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM acts GROUP BY number HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'Duplicate act numbers exist — resolve before applying this migration';
    END IF;
    IF EXISTS (SELECT 1 FROM invoices GROUP BY number HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'Duplicate invoice numbers exist — resolve before applying this migration';
    END IF;
END $$;

ALTER TABLE acts DROP CONSTRAINT acts_unique_number;
ALTER TABLE acts ADD CONSTRAINT acts_unique_number UNIQUE (number);

ALTER TABLE invoices DROP CONSTRAINT invoices_unique_number;
ALTER TABLE invoices ADD CONSTRAINT invoices_unique_number UNIQUE (number);
