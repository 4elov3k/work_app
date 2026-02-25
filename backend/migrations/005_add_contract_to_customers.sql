-- Добавление поля номера договора к контрагентам

ALTER TABLE customers ADD COLUMN contract_number VARCHAR(100) DEFAULT 'Основной';

-- Индекс для поиска по номеру договора
CREATE INDEX idx_customers_contract_number ON customers(contract_number);

-- Удаляем поле из таблицы invoices, так как оно теперь будет браться из customers
ALTER TABLE invoices DROP COLUMN IF EXISTS contract_number;
