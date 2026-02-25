-- Добавление типа документа (счет или акт)

-- Создаем ENUM тип для типа документа
CREATE TYPE document_type AS ENUM ('invoice', 'certificate');

-- Добавляем колонку document_type в таблицу invoices
ALTER TABLE invoices ADD COLUMN document_type document_type DEFAULT 'invoice';

-- Индекс для быстрой фильтрации по типу
CREATE INDEX idx_invoices_document_type ON invoices(document_type);

-- Обновляем существующие записи (опционально, по умолчанию уже 'invoice')
-- UPDATE invoices SET document_type = 'invoice' WHERE document_type IS NULL;
