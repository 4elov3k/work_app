-- Добавление поля номера договора

ALTER TABLE invoices ADD COLUMN contract_number VARCHAR(100) DEFAULT 'Основной';

-- Индекс для поиска по номеру договора
CREATE INDEX idx_invoices_contract_number ON invoices(contract_number);
