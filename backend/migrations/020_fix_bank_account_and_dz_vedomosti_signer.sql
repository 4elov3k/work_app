-- Corrects the seller's bank account (transposed digits) and updates the
-- Дзержинские ведомости signer to match the currently signed contract
-- (Договор №602 от 02.12.2025): и.о. директора Липатова Анастасия Павловна.

UPDATE organizations
SET bank_account = '40802810614270001108'
WHERE active = true
  AND bank_account = '40802810164270001108';

UPDATE customers
SET
    contact_person = 'Липатова Анастасия Павловна',
    contact_position = 'И.о. директора'
WHERE inn = '5249091492';
