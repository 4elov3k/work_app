package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"invoices-backend/internal/models"
)

// GetActs возвращает список актов с опциональной фильтрацией по контрагенту и договору
func (db *DB) GetActs(ctx context.Context, customerID, contractID string, archived *bool, page, perPage int) ([]models.Act, int, error) {
	var acts []models.Act
	var total int

	countQuery := "SELECT COUNT(*) FROM acts"
	var args []interface{}
	var conditions []string
	argCount := 1

	if contractID != "" {
		conditions = append(conditions, fmt.Sprintf("contract_id = $%d", argCount))
		args = append(args, contractID)
		argCount++
	} else if customerID != "" {
		conditions = append(conditions, fmt.Sprintf("contract_id IN (SELECT id FROM contracts WHERE customer_id = $%d)", argCount))
		args = append(args, customerID)
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

	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count acts: %w", err)
	}

	query := `
		SELECT a.id, a.contract_id, c.customer_id, a.number, a.date, a.status, a.total_amount,
		       a.archived, c.number AS contract_number, a.created_at, a.updated_at
		FROM acts a
		JOIN contracts c ON c.id = a.contract_id
	`

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY to_date(a.date, 'DD.MM.YYYY') DESC, a.number DESC"
	if perPage > 0 {
		offset := (page - 1) * perPage
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
		args = append(args, perPage, offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query acts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var act models.Act
		if err := rows.Scan(
			&act.ID,
			&act.ContractID,
			&act.CustomerID,
			&act.Number,
			&act.Date,
			&act.Status,
			&act.TotalAmount,
			&act.Archived,
			&act.ContractNumber,
			&act.CreatedAt,
			&act.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan act: %w", err)
		}
		acts = append(acts, act)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating acts: %w", err)
	}
	return acts, total, nil
}

// GetActByID возвращает акт по ID
func (db *DB) GetActByID(ctx context.Context, id string) (*models.Act, error) {
	query := `
		SELECT a.id, a.contract_id, c.customer_id, a.number, a.date, a.status, a.total_amount,
		       a.archived, c.number AS contract_number, a.created_at, a.updated_at
		FROM acts a
		JOIN contracts c ON c.id = a.contract_id
		WHERE a.id = $1
	`
	var act models.Act
	err := db.QueryRowContext(ctx, query, id).Scan(
		&act.ID,
		&act.ContractID,
		&act.CustomerID,
		&act.Number,
		&act.Date,
		&act.Status,
		&act.TotalAmount,
		&act.Archived,
		&act.ContractNumber,
		&act.CreatedAt,
		&act.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("act not found: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get act: %w", err)
	}
	return &act, nil
}

// GetActWithServices возвращает акт с услугами (по snapshot)
func (db *DB) GetActWithServices(ctx context.Context, id string) (*models.ActWithServices, error) {
	debug := os.Getenv("LOG_LEVEL") == "debug"

	act, err := db.GetActByID(ctx, id)
	if err != nil {
		if debug {
			log.Printf("[DEBUG] GetActByID failed: %v", err)
		}
		return nil, err
	}

	linesQuery := `
		SELECT id, title_snapshot, unit_snapshot, COALESCE(vat_snapshot, 0), price_snapshot, qty, amount
		FROM act_lines
		WHERE act_id = $1
		ORDER BY id
	`
	rows, err := db.QueryContext(ctx, linesQuery, id)
	if err != nil {
		if debug {
			log.Printf("[DEBUG] Query act_lines failed: %v", err)
		}
		return nil, fmt.Errorf("failed to query act lines: %w", err)
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
			return nil, fmt.Errorf("failed to scan act line: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating act lines: %w", err)
	}

	invoicesQuery := `
		SELECT i.id, i.contract_id, i.customer_id, i.number, i.date, i.status, i.total_amount,
		       i.archived, i.contract_number, i.created_at, i.updated_at
		FROM act_invoices ai
		JOIN invoices i ON i.id = ai.invoice_id
		WHERE ai.act_id = $1
		ORDER BY to_date(i.date, 'DD.MM.YYYY'), i.number
	`
	invoiceRows, err := db.QueryContext(ctx, invoicesQuery, id)
	if err != nil {
		if debug {
			log.Printf("[DEBUG] Query act invoices failed: %v", err)
		}
		return nil, fmt.Errorf("failed to query act invoices: %w", err)
	}
	defer invoiceRows.Close()

	var invoices []models.Invoice
	for invoiceRows.Next() {
		var invoice models.Invoice
		if err := invoiceRows.Scan(
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
		); err != nil {
			return nil, fmt.Errorf("failed to scan act invoice: %w", err)
		}
		invoices = append(invoices, invoice)
	}
	if err := invoiceRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating act invoices: %w", err)
	}

	return &models.ActWithServices{
		Act:      *act,
		Services: services,
		Invoices: invoices,
	}, nil
}

// CreateAct создает акт с услугами и связями со счетами
func (db *DB) CreateAct(ctx context.Context, req models.CreateActRequest) (*models.Act, error) {
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

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	actQuery := `
		INSERT INTO acts (contract_id, number, date, status, total_amount, archived)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`
	var act models.Act
	err = tx.QueryRowContext(ctx, actQuery, req.ContractID, req.Number, req.Date, defaultStatus(req.Status), 0, false).Scan(
		&act.ID,
		&act.ContractID,
		&act.Number,
		&act.Date,
		&act.Status,
		&act.TotalAmount,
		&act.Archived,
		&act.CreatedAt,
		&act.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create act: %w", err)
	}

	lines, totalAmount, err := buildInvoiceLines(ctx, tx, models.CreateInvoiceRequest{
		ServiceIDs: req.ServiceIDs,
		Services:   req.Services,
		Lines:      req.Lines,
	})
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("at least one line is required")
	}

	lineQuery := `
		INSERT INTO act_lines
			(act_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for _, line := range lines {
		_, err = tx.ExecContext(ctx, lineQuery, act.ID, line.ServiceID, line.Title, line.Unit, line.VAT, line.Price, line.Qty, line.Amount)
		if err != nil {
			return nil, fmt.Errorf("failed to create act line: %w", err)
		}
	}

	if len(req.InvoiceIDs) > 0 {
		linkQuery := `INSERT INTO act_invoices (act_id, invoice_id) VALUES ($1, $2)`
		for _, invoiceID := range req.InvoiceIDs {
			if _, err := tx.ExecContext(ctx, linkQuery, act.ID, invoiceID); err != nil {
				return nil, fmt.Errorf("failed to link act to invoice: %w", err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE acts SET total_amount = $1 WHERE id = $2`, totalAmount, act.ID); err != nil {
		return nil, fmt.Errorf("failed to update act total: %w", err)
	}
	act.TotalAmount = totalAmount

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	act.CustomerID = contract.CustomerID
	act.ContractNumber = contract.Number
	return &act, nil
}

// LinkActInvoices связывает акт с наборами счетов
func (db *DB) LinkActInvoices(ctx context.Context, actID string, invoiceIDs []string) error {
	if len(invoiceIDs) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	linkQuery := `INSERT INTO act_invoices (act_id, invoice_id) VALUES ($1, $2)`
	for _, invoiceID := range invoiceIDs {
		if _, err := tx.ExecContext(ctx, linkQuery, actID, invoiceID); err != nil {
			return fmt.Errorf("failed to link act to invoice: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// CheckActNumberExists проверяет уникальность номера акта в рамках договора
// CheckActNumberExists проверяет, занят ли номер акта. Номера актов
// уникальны глобально, поэтому contract_id в условие не входит; contractID
// сохранён в сигнатуре ради обратной совместимости вызовов.
func (db *DB) CheckActNumberExists(ctx context.Context, contractID, number string, excludeID string) (bool, error) {
	var query string
	var args []interface{}

	if excludeID == "" {
		query = `
			SELECT COUNT(*)
			FROM acts
			WHERE number = $1
		`
		args = []interface{}{number}
	} else {
		query = `
			SELECT COUNT(*)
			FROM acts
			WHERE number = $1 AND id != $2
		`
		args = []interface{}{number, excludeID}
	}

	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to check act number: %w", err)
	}
	return count > 0, nil
}

// DeleteAct удаляет акт по ID
func (db *DB) DeleteAct(ctx context.Context, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM acts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete act: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetNextActNumber возвращает следующий номер акта для договора
// GetNextActNumber возвращает следующий номер акта. Номера актов уникальны
// глобально (across всех договоров и клиентов), а не только в рамках одного
// договора — поэтому MAX считается по всей таблице. contractID сохранён в
// сигнатуре ради обратной совместимости вызовов, но в запросе не участвует.
func (db *DB) GetNextActNumber(ctx context.Context, contractID string) (int64, error) {
	query := `
		SELECT COALESCE(MAX(number::bigint), 2999) + 1
		FROM acts
		WHERE number ~ '^[0-9]+$'
	`
	var next int64
	if err := db.QueryRowContext(ctx, query).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to get next act number: %w", err)
	}
	return next, nil
}

// AddActLine добавляет строку в акт и обновляет сумму
func (db *DB) AddActLine(ctx context.Context, actID string, line models.InvoiceLineInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var archived bool
	if err := tx.QueryRowContext(ctx, `SELECT archived FROM acts WHERE id = $1`, actID).Scan(&archived); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to check act: %w", err)
	}
	if archived {
		return fmt.Errorf("act is archived")
	}

	snapshot, amount, err := buildSingleInvoiceLine(ctx, tx, line)
	if err != nil {
		return err
	}

	lineQuery := `
		INSERT INTO act_lines
			(act_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if _, err := tx.ExecContext(ctx, lineQuery, actID, snapshot.ServiceID, snapshot.Title, snapshot.Unit, snapshot.VAT, snapshot.Price, snapshot.Qty, snapshot.Amount); err != nil {
		return fmt.Errorf("failed to create act line: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE acts SET total_amount = total_amount + $1 WHERE id = $2`, amount, actID); err != nil {
		return fmt.Errorf("failed to update act total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// UpdateActLine обновляет строку акта и пересчитывает сумму.
func (db *DB) UpdateActLine(ctx context.Context, actID, lineID string, line models.InvoiceLineInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var archived bool
	if err := tx.QueryRowContext(ctx, `SELECT archived FROM acts WHERE id = $1`, actID).Scan(&archived); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to check act: %w", err)
	}
	if archived {
		return fmt.Errorf("act is archived")
	}

	snapshot, _, err := buildSingleInvoiceLine(ctx, tx, line)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE act_lines
		SET service_id = $1,
		    title_snapshot = $2,
		    unit_snapshot = $3,
		    vat_snapshot = $4,
		    price_snapshot = $5,
		    qty = $6,
		    amount = $7
		WHERE id = $8 AND act_id = $9
	`, snapshot.ServiceID, snapshot.Title, snapshot.Unit, snapshot.VAT, snapshot.Price, snapshot.Qty, snapshot.Amount, lineID, actID)
	if err != nil {
		return fmt.Errorf("failed to update act line: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE acts
		SET total_amount = COALESCE((SELECT SUM(amount) FROM act_lines WHERE act_id = $1), 0)
		WHERE id = $1
	`, actID); err != nil {
		return fmt.Errorf("failed to update act total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// DeleteActLine удаляет строку акта и пересчитывает сумму.
func (db *DB) DeleteActLine(ctx context.Context, actID, lineID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var archived bool
	if err := tx.QueryRowContext(ctx, `SELECT archived FROM acts WHERE id = $1`, actID).Scan(&archived); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to check act: %w", err)
	}
	if archived {
		return fmt.Errorf("act is archived")
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM act_lines WHERE id = $1 AND act_id = $2`, lineID, actID)
	if err != nil {
		return fmt.Errorf("failed to delete act line: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE acts
		SET total_amount = COALESCE((SELECT SUM(amount) FROM act_lines WHERE act_id = $1), 0)
		WHERE id = $1
	`, actID); err != nil {
		return fmt.Errorf("failed to update act total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// CreateActFromInvoice создает акт на основании счета
func (db *DB) CreateActFromInvoice(ctx context.Context, invoiceID, number, date, status string) (*models.Act, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var contractID string
	var customerID string
	if err := tx.QueryRowContext(ctx, `SELECT contract_id, customer_id FROM invoices WHERE id = $1`, invoiceID).Scan(&contractID, &customerID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invoice not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	actQuery := `
		INSERT INTO acts (contract_id, number, date, status, total_amount, archived)
		VALUES ($1, $2, $3, $4, 0, $5)
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`
	var act models.Act
	if err := tx.QueryRowContext(ctx, actQuery, contractID, number, date, defaultStatus(status), false).Scan(
		&act.ID,
		&act.ContractID,
		&act.Number,
		&act.Date,
		&act.Status,
		&act.TotalAmount,
		&act.Archived,
		&act.CreatedAt,
		&act.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create act: %w", err)
	}

	linesQuery := `
		INSERT INTO act_lines (act_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		SELECT $1, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount
		FROM invoice_lines
		WHERE invoice_id = $2
	`
	if _, err := tx.ExecContext(ctx, linesQuery, act.ID, invoiceID); err != nil {
		return nil, fmt.Errorf("failed to copy invoice lines: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO act_invoices (act_id, invoice_id) VALUES ($1, $2)`, act.ID, invoiceID); err != nil {
		return nil, fmt.Errorf("failed to link act to invoice: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE acts
		SET total_amount = COALESCE((SELECT SUM(amount) FROM act_lines WHERE act_id = $1), 0)
		WHERE id = $1
	`, act.ID); err != nil {
		return nil, fmt.Errorf("failed to update act total: %w", err)
	}

	var contractNumber string
	if err := tx.QueryRowContext(ctx, `SELECT number FROM contracts WHERE id = $1`, contractID).Scan(&contractNumber); err != nil {
		return nil, fmt.Errorf("failed to get contract number: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	act.CustomerID = customerID
	act.ContractNumber = contractNumber
	return &act, nil
}

// UpdateAct обновляет акт
func (db *DB) UpdateAct(ctx context.Context, id string, number *string, date *string, status *string, archived *bool) (*models.Act, error) {
	query := `
		UPDATE acts
		SET number = COALESCE($2, number),
		    date = COALESCE($3, date),
		    status = COALESCE($4, status),
		    archived = COALESCE($5, archived)
		WHERE id = $1
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`
	var act models.Act
	var contractID string
	err := db.QueryRowContext(ctx, query, id, number, date, status, archived).Scan(
		&act.ID,
		&contractID,
		&act.Number,
		&act.Date,
		&act.Status,
		&act.TotalAmount,
		&act.Archived,
		&act.CreatedAt,
		&act.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update act: %w", err)
	}
	// hydrate customer_id and contract_number
	act.ContractID = contractID
	if contractID != "" {
		var customerID string
		var contractNumber string
		if err := db.QueryRowContext(ctx, `SELECT customer_id, number FROM contracts WHERE id = $1`, contractID).Scan(&customerID, &contractNumber); err == nil {
			act.CustomerID = customerID
			act.ContractNumber = contractNumber
		}
	}
	return &act, nil
}
