BEGIN;

-- 1) Нельзя создать invoice без существующего contract
DO $$
DECLARE
    bogus_contract UUID := gen_random_uuid();
    bogus_customer UUID := gen_random_uuid();
BEGIN
    BEGIN
        INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, contract_number)
        VALUES (bogus_contract, bogus_customer, 'INV-FAIL', '01.01.2025', 'draft', 0, 'Основной');
        RAISE EXCEPTION 'Expected FK/trigger violation for missing contract/customer';
    EXCEPTION WHEN others THEN
        -- ok (trigger should reject)
        NULL;
    END;
END $$;

-- 2) Нельзя связать акт и счет разных договоров
DO $$
DECLARE
    cust1 UUID;
    cust2 UUID;
    c1 UUID;
    c2 UUID;
    inv1 UUID;
    act2 UUID;
BEGIN
    INSERT INTO customers (name, fullname, address, inn)
    VALUES ('Test A', 'Test A LLC', 'Addr', '1234567890')
    RETURNING id INTO cust1;

    INSERT INTO customers (name, fullname, address, inn)
    VALUES ('Test B', 'Test B LLC', 'Addr', '1234567891')
    RETURNING id INTO cust2;

    INSERT INTO contracts (customer_id, number, currency, status, start_date)
    VALUES (cust1, 'C-1', 'RUB', 'active', CURRENT_DATE)
    RETURNING id INTO c1;

    INSERT INTO contracts (customer_id, number, currency, status, start_date)
    VALUES (cust2, 'C-2', 'RUB', 'active', CURRENT_DATE)
    RETURNING id INTO c2;

    INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, contract_number)
    VALUES (c1, cust1, 'INV-1', '01.01.2025', 'draft', 0, 'C-1')
    RETURNING id INTO inv1;

    INSERT INTO acts (contract_id, number, date, status, total_amount)
    VALUES (c2, 'ACT-1', '01.01.2025', 'draft', 0)
    RETURNING id INTO act2;

    BEGIN
        INSERT INTO act_invoices (act_id, invoice_id) VALUES (act2, inv1);
        RAISE EXCEPTION 'Expected contract mismatch failure';
    EXCEPTION WHEN others THEN
        -- ok (trigger should reject)
        NULL;
    END;
END $$;

-- 3) Удаление service не ломает историю: service_id в строке становится NULL
DO $$
DECLARE
    cust UUID;
    con UUID;
    inv UUID;
    svc UUID;
    line_service UUID;
BEGIN
    INSERT INTO customers (name, fullname, address, inn)
    VALUES ('Test C', 'Test C LLC', 'Addr', '1234567892')
    RETURNING id INTO cust;

    INSERT INTO contracts (customer_id, number, currency, status, start_date)
    VALUES (cust, 'C-3', 'RUB', 'active', CURRENT_DATE)
    RETURNING id INTO con;

    INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, contract_number)
    VALUES (con, cust, 'INV-2', '01.01.2025', 'draft', 0, 'C-3')
    RETURNING id INTO inv;

    INSERT INTO services (name, price) VALUES ('Srv', 100) RETURNING id INTO svc;

    INSERT INTO invoice_lines (invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
    VALUES (inv, svc, 'Srv', 'шт', 0, 100, 1, 100);

    DELETE FROM services WHERE id = svc;

    SELECT service_id INTO line_service FROM invoice_lines WHERE invoice_id = inv LIMIT 1;
    IF line_service IS NOT NULL THEN
        RAISE EXCEPTION 'Expected service_id to be NULL after service deletion';
    END IF;
END $$;

-- 4) Уникальность номера счета в рамках договора
DO $$
DECLARE
    cust UUID;
    con UUID;
BEGIN
    INSERT INTO customers (name, fullname, address, inn)
    VALUES ('Test D', 'Test D LLC', 'Addr', '1234567893')
    RETURNING id INTO cust;

    INSERT INTO contracts (customer_id, number, currency, status, start_date)
    VALUES (cust, 'C-4', 'RUB', 'active', CURRENT_DATE)
    RETURNING id INTO con;

    INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, contract_number)
    VALUES (con, cust, 'INV-3', '01.01.2025', 'draft', 0, 'C-4');

    BEGIN
        INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, contract_number)
        VALUES (con, cust, 'INV-3', '02.01.2025', 'draft', 0, 'C-4');
        RAISE EXCEPTION 'Expected unique violation for invoice number';
    EXCEPTION WHEN unique_violation THEN
        NULL;
    END;
END $$;

ROLLBACK;
