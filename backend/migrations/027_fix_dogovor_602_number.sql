-- The contract for МАУ «ИЦ «Дзержинские ведомости» (real signed document
-- "ДОГОВОР №602 от 02.12.2025") had been entered into work_app under its own
-- internal sequential number (610) rather than the number printed on the
-- physical contract. Per explicit instruction, the printed/paper number MUST
-- match work_app's number — this was not a cosmetic detail, an app's own
-- sequential numbering should not diverge from the real document's number.
--
-- (This mismatch is also what caused migration 023 to create a phantom
-- duplicate contract "602" for the appendix instead of attaching it to this
-- real contract — the migration assumed no contract numbered "602" existed.)

UPDATE contracts SET number = '602'
WHERE id = 'c6019b38-6480-4800-a201-ef3c6a0dd55d';

UPDATE invoices i SET contract_number = c.number
FROM contracts c
WHERE i.contract_id = c.id
  AND i.contract_id = 'c6019b38-6480-4800-a201-ef3c6a0dd55d'
  AND i.contract_number IS DISTINCT FROM c.number;
