package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"invoices-backend/internal/models"
)

// GetInvoices возвращает список счетов с опциональной фильтрацией по контрагенту, договору и архиву
func (db *DB) GetInvoices(ctx context.Context, customerID, contractID string, archived *bool, page, perPage int) ([]models.Invoice, int, error) {
	var invoices []models.Invoice
	var total int

	// Подсчет общего количества
	countQuery := "SELECT COUNT(*) FROM invoices"
	var args []interface{}
	var conditions []string
	argCount := 1

	if customerID != "" {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argCount))
		args = append(args, customerID)
		argCount++
	}

	if contractID != "" {
		conditions = append(conditions, fmt.Sprintf("contract_id = $%d", argCount))
		args = append(args, contractID)
		argCount++
	}
	if archived != nil {
		conditions = append(conditions, fmt.Sprintf("archived = $%d", argCount))
		args = append(args, *archived)
		argCount++
	}

	if len(conditions) > 0 {
		countQuery += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			countQuery += " AND " + conditions[i]
		}
	}

	err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count invoices: %w", err)
	}

	// Получение данных с пагинацией
	query := `
		SELECT id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
		FROM invoices
	`

	args = nil
	conditions = nil
	argCount = 1

	if customerID != "" {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argCount))
		args = append(args, customerID)
		argCount++
	}

	if contractID != "" {
		conditions = append(conditions, fmt.Sprintf("contract_id = $%d", argCount))
		args = append(args, contractID)
		argCount++
	}
	if archived != nil {
		conditions = append(conditions, fmt.Sprintf("archived = $%d", argCount))
		args = append(args, *archived)
		argCount++
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY to_date(date, 'DD.MM.YYYY') DESC, number DESC"

	if perPage > 0 {
		offset := (page - 1) * perPage
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
		args = append(args, perPage, offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query invoices: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var invoice models.Invoice
		err := rows.Scan(
			&invoice.ID,
			&invoice.ContractID,
			&invoice.CustomerID,
			&invoice.Number,
			&invoice.Date,
			&invoice.Status,
			&invoice.TotalAmount,
			&invoice.Archived,
			&invoice.ContractNumber,
			&invoice.CreatedAt,
			&invoice.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan invoice: %w", err)
		}
		invoices = append(invoices, invoice)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating invoices: %w", err)
	}

	return invoices, total, nil
}

// GetInvoiceByID возвращает счет по ID
func (db *DB) GetInvoiceByID(ctx context.Context, id string) (*models.Invoice, error) {
	query := `
		SELECT id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
		FROM invoices
		WHERE id = $1
	`

	var invoice models.Invoice
	err := db.QueryRowContext(ctx, query, id).Scan(
		&invoice.ID,
		&invoice.ContractID,
		&invoice.CustomerID,
		&invoice.Number,
		&invoice.Date,
		&invoice.Status,
		&invoice.TotalAmount,
		&invoice.Archived,
		&invoice.ContractNumber,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invoice not found: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	return &invoice, nil
}

// GetInvoiceWithServices возвращает счет с услугами
func (db *DB) GetInvoiceWithServices(ctx context.Context, id string) (*models.InvoiceWithServices, error) {
	debug := os.Getenv("LOG_LEVEL") == "debug"

	// Получаем счет
	invoice, err := db.GetInvoiceByID(ctx, id)
	if err != nil {
		if debug {
			log.Printf("[DEBUG] GetInvoiceByID failed: %v", err)
		}
		return nil, err
	}
	if debug {
		log.Printf("[DEBUG] Invoice found: %s", invoice.Number)
	}

	// Получаем строки счета и восстанавливаем услуги из snapshot
	linesQuery := `
		SELECT id, title_snapshot, unit_snapshot, COALESCE(vat_snapshot, 0), price_snapshot, qty, amount
		FROM invoice_lines
		WHERE invoice_id = $1
		ORDER BY id
	`

	rows, err := db.QueryContext(ctx, linesQuery, id)
	if err != nil {
		if debug {
			log.Printf("[DEBUG] Query invoice_lines failed: %v", err)
		}
		return nil, fmt.Errorf("failed to query invoice lines: %w", err)
	}
	defer rows.Close()

	var services []models.Service
	for rows.Next() {
		var lineID string
		var title string
		var unit string
		var vat float64
		var price float64
		var qty float64
		var amount float64
		if err := rows.Scan(&lineID, &title, &unit, &vat, &price, &qty, &amount); err != nil {
			if debug {
				log.Printf("[DEBUG] Scan invoice line failed: %v", err)
			}
			return nil, fmt.Errorf("failed to scan invoice line: %w", err)
		}
		services = append(services, models.Service{
			ID:     lineID,
			Name:   title,
			Unit:   unit,
			VAT:    vat,
			Price:  price,
			Qty:    qty,
			Amount: amount,
		})
	}
	if debug {
		log.Printf("[DEBUG] Found %d invoice lines", len(services))
	}

	if err = rows.Err(); err != nil {
		if debug {
			log.Printf("[DEBUG] Rows error: %v", err)
		}
		return nil, fmt.Errorf("error iterating service IDs: %w", err)
	}

	return &models.InvoiceWithServices{
		Invoice:  *invoice,
		Services: services,
	}, nil
}

// DeleteInvoice удаляет счет по ID
func (db *DB) DeleteInvoice(ctx context.Context, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM invoices WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete invoice: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetNextInvoiceNumber возвращает следующий номер счета для договора
// GetNextInvoiceNumber возвращает следующий номер счёта. Номера счетов
// уникальны глобально (across всех договоров и клиентов), а не только в
// рамках одного договора — поэтому MAX считается по всей таблице. contractID
// сохранён в сигнатуре ради обратной совместимости вызовов, но в запросе не
// участвует.
func (db *DB) GetNextInvoiceNumber(ctx context.Context, contractID string) (int64, error) {
	query := `
		SELECT COALESCE(MAX(number::bigint), 2999) + 1
		FROM invoices
		WHERE number ~ '^[0-9]+$'
	`
	var next int64
	if err := db.QueryRowContext(ctx, query).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to get next invoice number: %w", err)
	}
	return next, nil
}

// CreateInvoice создает новый счет с услугами в транзакции
func (db *DB) CreateInvoice(ctx context.Context, req models.CreateInvoiceRequest) (*models.Invoice, error) {
	// Разрешаем создание только в рамках существующего договора
	var contract *models.Contract
	var err error
	if req.ContractID != "" {
		contract, err = db.GetContractByID(ctx, req.ContractID)
		if err != nil {
			return nil, fmt.Errorf("contract not found: %w", ErrNotFound)
		}
	} else if req.CustomerID != "" && req.ContractNumber != "" {
		contract, err = db.GetContractByCustomerAndNumber(ctx, req.CustomerID, req.ContractNumber)
		if err != nil {
			return nil, fmt.Errorf("contract not found: %w", ErrNotFound)
		}
		req.ContractID = contract.ID
	} else {
		return nil, fmt.Errorf("contract_id is required")
	}

	// Начинаем транзакцию
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Гарантируем согласованность customer_id
	customerID := contract.CustomerID
	contractNumber := contract.Number

	// Создаем счет
	invoiceQuery := `
		INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, archived, contract_number)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
	`

	var invoice models.Invoice
	err = tx.QueryRowContext(ctx, invoiceQuery, req.ContractID, customerID, req.Number, req.Date, defaultStatus(req.Status), 0, false, contractNumber).Scan(
		&invoice.ID,
		&invoice.ContractID,
		&invoice.CustomerID,
		&invoice.Number,
		&invoice.Date,
		&invoice.Status,
		&invoice.TotalAmount,
		&invoice.Archived,
		&invoice.ContractNumber,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	lines, totalAmount, err := buildInvoiceLines(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("at least one line is required")
	}

	lineQuery := `
		INSERT INTO invoice_lines
			(invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for _, line := range lines {
		_, err = tx.ExecContext(ctx, lineQuery, invoice.ID, line.ServiceID, line.Title, line.Unit, line.VAT, line.Price, line.Qty, line.Amount)
		if err != nil {
			return nil, fmt.Errorf("failed to create invoice line: %w", err)
		}
	}

	updateTotalQuery := `UPDATE invoices SET total_amount = $1 WHERE id = $2`
	if _, err := tx.ExecContext(ctx, updateTotalQuery, totalAmount, invoice.ID); err != nil {
		return nil, fmt.Errorf("failed to update invoice total: %w", err)
	}
	invoice.TotalAmount = totalAmount

	// Подтверждаем транзакцию
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &invoice, nil
}

// DuplicateInvoice дублирует существующий счет с новой датой и номером
func (db *DB) DuplicateInvoice(ctx context.Context, req models.DuplicateInvoiceRequest) (*models.Invoice, error) {
	// Начинаем транзакцию
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Получаем оригинальный счет
	originalQuery := `
		SELECT contract_id, customer_id, contract_number
		FROM invoices
		WHERE id = $1
	`
	var customerID string
	var contractID string
	var contractNumber string
	err = tx.QueryRowContext(ctx, originalQuery, req.InvoiceID).Scan(&contractID, &customerID, &contractNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get original invoice: %w", err)
	}

	// Проверяем уникальность номера для клиента (без исключения, т.к. новый документ)
	var count int
	checkQuery := `
		SELECT COUNT(*)
		FROM invoices
		WHERE contract_id = $1 AND number = $2
	`
	if err := tx.QueryRowContext(ctx, checkQuery, contractID, req.Number).Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to check invoice number: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("invoice number already exists for this contract")
	}

	// Создаем новый счет
	invoiceQuery := `
		INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, archived, contract_number)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
	`

	var newInvoice models.Invoice
	err = tx.QueryRowContext(ctx, invoiceQuery, contractID, customerID, req.Number, req.Date, "draft", 0, false, contractNumber).Scan(
		&newInvoice.ID,
		&newInvoice.ContractID,
		&newInvoice.CustomerID,
		&newInvoice.Number,
		&newInvoice.Date,
		&newInvoice.Status,
		&newInvoice.TotalAmount,
		&newInvoice.Archived,
		&newInvoice.ContractNumber,
		&newInvoice.CreatedAt,
		&newInvoice.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create duplicated invoice: %w", err)
	}

	// Копируем строки счета
	copyLinesQuery := `
		INSERT INTO invoice_lines (invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		SELECT $1, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount
		FROM invoice_lines
		WHERE invoice_id = $2
	`
	_, err = tx.ExecContext(ctx, copyLinesQuery, newInvoice.ID, req.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to copy invoice lines: %w", err)
	}

	updateTotalQuery := `
		UPDATE invoices
		SET total_amount = sub.total
		FROM (
			SELECT invoice_id, COALESCE(SUM(amount), 0) AS total
			FROM invoice_lines
			WHERE invoice_id = $1
			GROUP BY invoice_id
		) sub
		WHERE invoices.id = sub.invoice_id
	`
	if _, err := tx.ExecContext(ctx, updateTotalQuery, newInvoice.ID); err != nil {
		return nil, fmt.Errorf("failed to update duplicated invoice total: %w", err)
	}

	// Подтверждаем транзакцию
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &newInvoice, nil
}

// CheckInvoiceNumberExists проверяет существование номера счета у контрагента
// CheckInvoiceNumberExists проверяет, занят ли номер счёта. Номера счетов
// уникальны глобально, поэтому contract_id в условие не входит; contractID
// сохранён в сигнатуре ради обратной совместимости вызовов.
func (db *DB) CheckInvoiceNumberExists(ctx context.Context, contractID, number string, excludeID string) (bool, error) {
	var query string
	var args []interface{}

	if excludeID == "" {
		query = `
			SELECT COUNT(*)
			FROM invoices
			WHERE number = $1
		`
		args = []interface{}{number}
	} else {
		query = `
			SELECT COUNT(*)
			FROM invoices
			WHERE number = $1 AND id != $2
		`
		args = []interface{}{number, excludeID}
	}

	var count int
	err := db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check invoice number: %w", err)
	}

	return count > 0, nil
}

type invoiceLineSnapshot struct {
	ServiceID sql.NullString
	Title     string
	Unit      string
	VAT       float64
	Price     float64
	Qty       float64
	Amount    float64
}

func defaultStatus(status string) string {
	if status == "" {
		return "draft"
	}
	return status
}

func buildInvoiceLines(ctx context.Context, tx *sql.Tx, req models.CreateInvoiceRequest) ([]invoiceLineSnapshot, float64, error) {
	var lines []invoiceLineSnapshot
	total := 0.0

	for _, line := range req.Lines {
		if line.Qty == 0 {
			line.Qty = 1
		}
		if line.Unit == "" {
			line.Unit = "шт"
		}
		title := line.Title
		price := line.Price
		var serviceID sql.NullString

		if line.ServiceID != "" {
			service, err := getServiceByIDTx(ctx, tx, line.ServiceID)
			if err != nil {
				return nil, 0, fmt.Errorf("service not found: %w", ErrNotFound)
			}
			title = service.Name
			price = service.Price
			serviceID = sql.NullString{String: service.ID, Valid: true}
		}

		if title == "" || price < 0 || line.Qty <= 0 {
			return nil, 0, fmt.Errorf("invalid invoice line")
		}

		amount := price * line.Qty
		total += amount
		lines = append(lines, invoiceLineSnapshot{
			ServiceID: serviceID,
			Title:     title,
			Unit:      line.Unit,
			VAT:       line.VAT,
			Price:     price,
			Qty:       line.Qty,
			Amount:    amount,
		})
	}

	// service_ids (catalog references)
	if len(req.ServiceIDs) > 0 {
		services, err := getServicesByIDsTx(ctx, tx, req.ServiceIDs)
		if err != nil {
			return nil, 0, err
		}
		if len(services) != len(req.ServiceIDs) {
			return nil, 0, fmt.Errorf("one or more services not found: %w", ErrNotFound)
		}
		for _, s := range services {
			amount := s.Price
			total += amount
			lines = append(lines, invoiceLineSnapshot{
				ServiceID: sql.NullString{String: s.ID, Valid: true},
				Title:     s.Name,
				Unit:      "шт",
				VAT:       0,
				Price:     s.Price,
				Qty:       1,
				Amount:    amount,
			})
		}
	}

	// services payload (create new)
	if len(req.Services) > 0 {
		serviceQuery := `
			INSERT INTO services (name, price)
			VALUES ($1, $2)
			RETURNING id
		`
		for _, s := range req.Services {
			if s.Name == "" || s.Price <= 0 {
				return nil, 0, fmt.Errorf("invalid service")
			}
			var serviceID string
			if err := tx.QueryRowContext(ctx, serviceQuery, s.Name, s.Price).Scan(&serviceID); err != nil {
				return nil, 0, fmt.Errorf("failed to create service in invoice tx: %w", err)
			}
			amount := s.Price
			total += amount
			lines = append(lines, invoiceLineSnapshot{
				ServiceID: sql.NullString{String: serviceID, Valid: true},
				Title:     s.Name,
				Unit:      "шт",
				VAT:       0,
				Price:     s.Price,
				Qty:       1,
				Amount:    amount,
			})
		}
	}

	return lines, total, nil
}

func buildSingleInvoiceLine(ctx context.Context, tx *sql.Tx, line models.InvoiceLineInput) (invoiceLineSnapshot, float64, error) {
	req := models.CreateInvoiceRequest{
		Lines: []models.InvoiceLineInput{line},
	}
	lines, total, err := buildInvoiceLines(ctx, tx, req)
	if err != nil {
		return invoiceLineSnapshot{}, 0, err
	}
	if len(lines) == 0 {
		return invoiceLineSnapshot{}, 0, fmt.Errorf("invalid invoice line")
	}
	return lines[0], total, nil
}

// AddInvoiceLine добавляет строку в счет и обновляет сумму
func (db *DB) AddInvoiceLine(ctx context.Context, invoiceID string, line models.InvoiceLineInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var archived bool
	if err := tx.QueryRowContext(ctx, `SELECT archived FROM invoices WHERE id = $1`, invoiceID).Scan(&archived); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to check invoice: %w", err)
	}
	if archived {
		return fmt.Errorf("invoice is archived")
	}

	snapshot, amount, err := buildSingleInvoiceLine(ctx, tx, line)
	if err != nil {
		return err
	}

	lineQuery := `
		INSERT INTO invoice_lines
			(invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if _, err := tx.ExecContext(ctx, lineQuery, invoiceID, snapshot.ServiceID, snapshot.Title, snapshot.Unit, snapshot.VAT, snapshot.Price, snapshot.Qty, snapshot.Amount); err != nil {
		return fmt.Errorf("failed to create invoice line: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE invoices SET total_amount = total_amount + $1 WHERE id = $2`, amount, invoiceID); err != nil {
		return fmt.Errorf("failed to update invoice total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// UpdateInvoiceLine обновляет строку счета и пересчитывает сумму.
func (db *DB) UpdateInvoiceLine(ctx context.Context, invoiceID, lineID string, line models.InvoiceLineInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var archived bool
	if err := tx.QueryRowContext(ctx, `SELECT archived FROM invoices WHERE id = $1`, invoiceID).Scan(&archived); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to check invoice: %w", err)
	}
	if archived {
		return fmt.Errorf("invoice is archived")
	}

	snapshot, _, err := buildSingleInvoiceLine(ctx, tx, line)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE invoice_lines
		SET service_id = $1,
		    title_snapshot = $2,
		    unit_snapshot = $3,
		    vat_snapshot = $4,
		    price_snapshot = $5,
		    qty = $6,
		    amount = $7
		WHERE id = $8 AND invoice_id = $9
	`, snapshot.ServiceID, snapshot.Title, snapshot.Unit, snapshot.VAT, snapshot.Price, snapshot.Qty, snapshot.Amount, lineID, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to update invoice line: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE invoices
		SET total_amount = COALESCE((SELECT SUM(amount) FROM invoice_lines WHERE invoice_id = $1), 0)
		WHERE id = $1
	`, invoiceID); err != nil {
		return fmt.Errorf("failed to update invoice total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// DeleteInvoiceLine удаляет строку счета и пересчитывает сумму.
func (db *DB) DeleteInvoiceLine(ctx context.Context, invoiceID, lineID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var archived bool
	if err := tx.QueryRowContext(ctx, `SELECT archived FROM invoices WHERE id = $1`, invoiceID).Scan(&archived); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to check invoice: %w", err)
	}
	if archived {
		return fmt.Errorf("invoice is archived")
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM invoice_lines WHERE id = $1 AND invoice_id = $2`, lineID, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to delete invoice line: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE invoices
		SET total_amount = COALESCE((SELECT SUM(amount) FROM invoice_lines WHERE invoice_id = $1), 0)
		WHERE id = $1
	`, invoiceID); err != nil {
		return fmt.Errorf("failed to update invoice total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func getServiceByIDTx(ctx context.Context, tx *sql.Tx, id string) (*models.Service, error) {
	query := `SELECT id, name, price, created_at, updated_at FROM services WHERE id = $1`
	var s models.Service
	if err := tx.QueryRowContext(ctx, query, id).Scan(&s.ID, &s.Name, &s.Price, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service not found: %w", ErrNotFound)
		}
		return nil, err
	}
	return &s, nil
}

func getServicesByIDsTx(ctx context.Context, tx *sql.Tx, ids []string) ([]models.Service, error) {
	if len(ids) == 0 {
		return []models.Service{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, name, price, created_at, updated_at
		FROM services
		WHERE id IN (%s)
	`, joinStrings(placeholders, ","))

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query services: %w", err)
	}
	defer rows.Close()

	var services []models.Service
	for rows.Next() {
		var s models.Service
		if err := rows.Scan(&s.ID, &s.Name, &s.Price, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan service: %w", err)
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}
	return services, nil
}

// UpdateInvoice обновляет счет
func (db *DB) UpdateInvoice(ctx context.Context, id string, number *string, date *string, status *string, archived *bool) (*models.Invoice, error) {
	query := `
		UPDATE invoices
		SET number = COALESCE($2, number),
		    date = COALESCE($3, date),
		    status = COALESCE($4, status),
		    archived = COALESCE($5, archived)
		WHERE id = $1
		RETURNING id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
	`
	var invoice models.Invoice
	err := db.QueryRowContext(ctx, query, id, number, date, status, archived).Scan(
		&invoice.ID,
		&invoice.ContractID,
		&invoice.CustomerID,
		&invoice.Number,
		&invoice.Date,
		&invoice.Status,
		&invoice.TotalAmount,
		&invoice.Archived,
		&invoice.ContractNumber,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update invoice: %w", err)
	}
	return &invoice, nil
}

func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += sep + items[i]
	}
	return out
}
