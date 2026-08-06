-- Repairs data created by the old (pre-fix) backend build that was still
-- running in production when a duplicate "Дзержинские ведомости" customer
-- record got created via the removed CreateDefaultContractTx auto-provisioning
-- path (number='Основной', topic='Продвижение сео', empty signer).

-- 1. Re-apply the same generic non-numeric-contract-number repair as
--    migration 018 — a safety net in case any other contract slipped through
--    with the old placeholder number before the fixed backend was deployed.
UPDATE contracts c
SET number = m.extracted
FROM (
    SELECT
        c2.id,
        c2.customer_id,
        (regexp_match(c2.number, '№\s*([0-9]+)'))[1] AS extracted
    FROM contracts c2
    WHERE c2.number !~ '^[0-9]+$'
) m
WHERE c.id = m.id
  AND m.extracted IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM contracts c4
      WHERE c4.customer_id = m.customer_id
        AND c4.number = m.extracted
        AND c4.id <> m.id
  );

UPDATE contracts c
SET number = sub.next_number::text
FROM (
    SELECT
        c2.id,
        COALESCE(
            (SELECT MAX(c3.number::bigint)
             FROM contracts c3
             WHERE c3.customer_id = c2.customer_id
               AND c3.number ~ '^[0-9]+$'),
            699
        ) + 1 AS next_number
    FROM contracts c2
    WHERE c2.number !~ '^[0-9]+$'
) sub
WHERE c.id = sub.id;

UPDATE invoices i
SET contract_number = c.number
FROM contracts c
WHERE i.contract_id = c.id
  AND i.contract_number IS DISTINCT FROM c.number;

-- 2. Fill in the signer for the duplicate Дзержинские ведомости record
--    (same person as the existing card, per Договор №602: и.о. директора
--    Липатова Анастасия Павловна).
UPDATE customers
SET
    contact_person = 'Липатова Анастасия Павловна',
    contact_position = 'И.о. директора'
WHERE inn = '5249091492'
  AND COALESCE(contact_person, '') = '';
