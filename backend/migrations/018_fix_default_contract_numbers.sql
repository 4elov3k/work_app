-- Repairs contracts whose "number" column ended up holding descriptive text
-- instead of a plain sequential number:
--
--   1. Contracts auto-created with the literal placeholder 'Основной' (see
--      CreateDefaultContractTx, now removed — contracts are no longer
--      auto-created when a customer is registered). These get the next
--      available numeric contract number for their customer, using the
--      same scheme as GetNextContractNumber (starts at 700).
--
--   2. Legacy contracts whose number embeds the real number as free text,
--      e.g. 'Основной № 380 от 02.02.2022 г.' — the digits after "№" are
--      extracted (matching the same pattern export/updxml's
--      contractDocumentNumber() already parses at read time) and used as
--      the real number, as long as that number isn't already taken by
--      another contract for the same customer.

-- Case 2: extract the embedded "№ <digits>" number where present and safe.
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

-- Case 1: anything still non-numeric (bare 'Основной', or an embedded number
-- that collided with an existing contract) gets the next sequential number.
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

-- Re-sync the denormalized invoices.contract_number snapshot (a plain copy of
-- contracts.number taken at invoice-creation time, not an intentional
-- historical record) for any invoice left pointing at the old bad text.
UPDATE invoices i
SET contract_number = c.number
FROM contracts c
WHERE i.contract_id = c.id
  AND i.contract_number IS DISTINCT FROM c.number;
