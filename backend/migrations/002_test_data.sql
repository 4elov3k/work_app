-- Тестовые данные для разработки и тестирования

-- Добавление тестовых контрагентов
INSERT INTO customers (name, fullname, address, inn) VALUES
('ООО Ромашка', 'Общество с ограниченной ответственностью "Ромашка"', 'г. Москва, ул. Цветочная, д. 5', '7701234567'),
('ИП Иванов', 'Индивидуальный предприниматель Иванов Иван Иванович', 'г. Санкт-Петербург, пр. Невский, д. 100', '780198765432'),
('ООО Технопарк', 'Общество с ограниченной ответственностью "Технопарк"', 'г. Казань, ул. Баумана, д. 15', '1659876543');

-- Добавление тестовых услуг
INSERT INTO services (name, price) VALUES
('Консультация', 5000.00),
('Разработка сайта', 50000.00),
('Техническая поддержка', 10000.00),
('SEO оптимизация', 15000.00),
('Дизайн логотипа', 8000.00);

-- Получаем ID созданных записей для создания счетов
DO $$
DECLARE
    customer1_id UUID;
    customer2_id UUID;
    service1_id UUID;
    service2_id UUID;
    service3_id UUID;
    invoice1_id UUID;
    invoice2_id UUID;
BEGIN
    -- Получаем ID контрагентов
    SELECT id INTO customer1_id FROM customers WHERE inn = '7701234567';
    SELECT id INTO customer2_id FROM customers WHERE inn = '780198765432';
    
    -- Получаем ID услуг
    SELECT id INTO service1_id FROM services WHERE name = 'Консультация';
    SELECT id INTO service2_id FROM services WHERE name = 'Разработка сайта';
    SELECT id INTO service3_id FROM services WHERE name = 'Техническая поддержка';
    
    -- Создаем счета
    INSERT INTO invoices (customer_id, number, date)
    VALUES (customer1_id, '001', '10.01.2026')
    RETURNING id INTO invoice1_id;
    
    INSERT INTO invoices (customer_id, number, date)
    VALUES (customer2_id, '002', '11.01.2026')
    RETURNING id INTO invoice2_id;
    
    -- Связываем счета с услугами
    INSERT INTO invoice_services (invoice_id, service_id) VALUES
    (invoice1_id, service1_id),
    (invoice1_id, service2_id),
    (invoice2_id, service3_id);
END $$;
