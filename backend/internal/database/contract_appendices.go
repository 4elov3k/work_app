package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"invoices-backend/internal/models"
)

// resolveAppendixLine resolves one input line against the services catalog
// (when ServiceID is set, catalog values are used as defaults but explicit
// Title/Unit/Price/Qty in the input win) and computes its amount.
func resolveAppendixLine(ctx context.Context, tx *sql.Tx, input models.ContractAppendixLineInput) (models.ContractAppendixLine, error) {
	line := models.ContractAppendixLine{
		ServiceID: input.ServiceID,
		Section:   input.Section,
		Title:     strings.TrimSpace(input.Title),
		Unit:      input.Unit,
		Price:     input.Price,
		Qty:       input.Qty,
	}

	if input.ServiceID != "" {
		var name, unit, section string
		var price float64
		err := tx.QueryRowContext(ctx, `SELECT name, unit, section, price FROM services WHERE id = $1`, input.ServiceID).
			Scan(&name, &unit, &section, &price)
		if err == sql.ErrNoRows {
			return line, fmt.Errorf("service not found: %w", ErrNotFound)
		}
		if err != nil {
			return line, fmt.Errorf("failed to load service: %w", err)
		}
		if line.Title == "" {
			line.Title = name
		}
		if line.Unit == "" {
			line.Unit = unit
		}
		if line.Section == "" {
			line.Section = section
		}
		if line.Price == 0 {
			line.Price = price
		}
	}

	if line.Title == "" {
		return line, fmt.Errorf("line title is required")
	}
	if line.Unit == "" {
		line.Unit = "услуга"
	}
	if line.Qty == 0 {
		line.Qty = 1
	}
	if line.Qty < 0 {
		return line, fmt.Errorf("qty must be positive")
	}
	if line.Price < 0 {
		return line, fmt.Errorf("price must not be negative")
	}
	line.Amount = round2(line.Price * line.Qty)
	return line, nil
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// GetNextContractAppendixNumber возвращает следующий номер приложения для договора.
func (db *DB) GetNextContractAppendixNumber(ctx context.Context, contractID string) (int64, error) {
	var next int64
	query := `
		SELECT COALESCE(MAX(number::bigint), 0) + 1
		FROM contract_appendices
		WHERE contract_id = $1
		  AND number ~ '^[0-9]+$'
	`
	if err := db.QueryRowContext(ctx, query, contractID).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to get next appendix number: %w", err)
	}
	return next, nil
}

// CreateContractAppendix создает приложение к договору со строками в рамках транзакции.
func (db *DB) CreateContractAppendix(ctx context.Context, req models.CreateContractAppendixRequest) (*models.ContractAppendixWithLines, error) {
	if req.ContractID == "" {
		return nil, fmt.Errorf("contract_id is required")
	}
	if len(req.Lines) == 0 {
		return nil, fmt.Errorf("at least one line is required")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var contractExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM contracts WHERE id = $1)`, req.ContractID).Scan(&contractExists); err != nil {
		return nil, fmt.Errorf("failed to check contract: %w", err)
	}
	if !contractExists {
		return nil, fmt.Errorf("contract not found: %w", ErrNotFound)
	}

	lines := make([]models.ContractAppendixLine, 0, len(req.Lines))
	var total float64
	for i, input := range req.Lines {
		line, err := resolveAppendixLine(ctx, tx, input)
		if err != nil {
			return nil, err
		}
		line.Position = i + 1
		lines = append(lines, line)
		total += line.Amount
	}
	total = round2(total)

	var appendix models.ContractAppendix
	appendixQuery := `
		INSERT INTO contract_appendices (contract_id, number, date, status, total_amount, archived)
		VALUES ($1, $2, $3, $4, $5, false)
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, appendixQuery, req.ContractID, req.Number, req.Date, defaultStatus(req.Status), total).Scan(
		&appendix.ID,
		&appendix.ContractID,
		&appendix.Number,
		&appendix.Date,
		&appendix.Status,
		&appendix.TotalAmount,
		&appendix.Archived,
		&appendix.CreatedAt,
		&appendix.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract appendix: %w", err)
	}

	lineQuery := `
		INSERT INTO contract_appendix_lines
			(appendix_id, service_id, section, position, title_snapshot, unit_snapshot, price_snapshot, qty, amount)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	for i := range lines {
		if err := tx.QueryRowContext(ctx, lineQuery,
			appendix.ID, lines[i].ServiceID, lines[i].Section, lines[i].Position,
			lines[i].Title, lines[i].Unit, lines[i].Price, lines[i].Qty, lines[i].Amount,
		).Scan(&lines[i].ID); err != nil {
			return nil, fmt.Errorf("failed to create appendix line: %w", err)
		}
		lines[i].AppendixID = appendix.ID
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.ContractAppendixWithLines{ContractAppendix: appendix, Lines: lines}, nil
}

// GetContractAppendixWithLines возвращает приложение вместе со строками.
func (db *DB) GetContractAppendixWithLines(ctx context.Context, id string) (*models.ContractAppendixWithLines, error) {
	var appendix models.ContractAppendix
	err := db.QueryRowContext(ctx, `
		SELECT id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
		FROM contract_appendices
		WHERE id = $1
	`, id).Scan(
		&appendix.ID,
		&appendix.ContractID,
		&appendix.Number,
		&appendix.Date,
		&appendix.Status,
		&appendix.TotalAmount,
		&appendix.Archived,
		&appendix.CreatedAt,
		&appendix.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("contract appendix not found: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contract appendix: %w", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, appendix_id, COALESCE(service_id::text, ''), section, position, title_snapshot, unit_snapshot, price_snapshot, qty, amount
		FROM contract_appendix_lines
		WHERE appendix_id = $1
		ORDER BY position, created_at
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query appendix lines: %w", err)
	}
	defer rows.Close()

	var lines []models.ContractAppendixLine
	for rows.Next() {
		var line models.ContractAppendixLine
		if err := rows.Scan(
			&line.ID, &line.AppendixID, &line.ServiceID, &line.Section, &line.Position,
			&line.Title, &line.Unit, &line.Price, &line.Qty, &line.Amount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan appendix line: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating appendix lines: %w", err)
	}

	return &models.ContractAppendixWithLines{ContractAppendix: appendix, Lines: lines}, nil
}

// GetContractAppendicesByContract возвращает список приложений для договора.
func (db *DB) GetContractAppendicesByContract(ctx context.Context, contractID string) ([]models.ContractAppendix, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
		FROM contract_appendices
		WHERE contract_id = $1
		ORDER BY created_at DESC
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to query contract appendices: %w", err)
	}
	defer rows.Close()

	var appendices []models.ContractAppendix
	for rows.Next() {
		var a models.ContractAppendix
		if err := rows.Scan(&a.ID, &a.ContractID, &a.Number, &a.Date, &a.Status, &a.TotalAmount, &a.Archived, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan contract appendix: %w", err)
		}
		appendices = append(appendices, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contract appendices: %w", err)
	}
	return appendices, nil
}

// UpdateContractAppendix обновляет приложение (номер/дату/статус/архивность).
func (db *DB) UpdateContractAppendix(ctx context.Context, id string, req models.UpdateContractAppendixRequest) (*models.ContractAppendix, error) {
	query := `
		UPDATE contract_appendices
		SET
			number = COALESCE($2, number),
			date = COALESCE($3, date),
			status = COALESCE($4, status),
			archived = COALESCE($5, archived)
		WHERE id = $1
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`
	var a models.ContractAppendix
	err := db.QueryRowContext(ctx, query, id, req.Number, req.Date, req.Status, req.Archived).Scan(
		&a.ID, &a.ContractID, &a.Number, &a.Date, &a.Status, &a.TotalAmount, &a.Archived, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("contract appendix not found: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update contract appendix: %w", err)
	}
	return &a, nil
}

// DeleteContractAppendix удаляет приложение по ID.
func (db *DB) DeleteContractAppendix(ctx context.Context, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM contract_appendices WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete contract appendix: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// AddContractAppendixLine добавляет строку в приложение и пересчитывает сумму.
func (db *DB) AddContractAppendixLine(ctx context.Context, appendixID string, input models.ContractAppendixLineInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM contract_appendices WHERE id = $1)`, appendixID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check contract appendix: %w", err)
	}
	if !exists {
		return sql.ErrNoRows
	}

	var nextPosition int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM contract_appendix_lines WHERE appendix_id = $1`, appendixID).Scan(&nextPosition); err != nil {
		return fmt.Errorf("failed to determine line position: %w", err)
	}

	line, err := resolveAppendixLine(ctx, tx, input)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_appendix_lines
			(appendix_id, service_id, section, position, title_snapshot, unit_snapshot, price_snapshot, qty, amount)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9)
	`, appendixID, line.ServiceID, line.Section, nextPosition, line.Title, line.Unit, line.Price, line.Qty, line.Amount); err != nil {
		return fmt.Errorf("failed to create appendix line: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE contract_appendices SET total_amount = total_amount + $1 WHERE id = $2`, line.Amount, appendixID); err != nil {
		return fmt.Errorf("failed to update appendix total: %w", err)
	}

	return tx.Commit()
}

// DeleteContractAppendixLine удаляет строку приложения и пересчитывает сумму.
func (db *DB) DeleteContractAppendixLine(ctx context.Context, appendixID, lineID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var amount float64
	if err := tx.QueryRowContext(ctx, `SELECT amount FROM contract_appendix_lines WHERE id = $1 AND appendix_id = $2`, lineID, appendixID).Scan(&amount); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to load appendix line: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM contract_appendix_lines WHERE id = $1`, lineID); err != nil {
		return fmt.Errorf("failed to delete appendix line: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contract_appendices SET total_amount = total_amount - $1 WHERE id = $2`, amount, appendixID); err != nil {
		return fmt.Errorf("failed to update appendix total: %w", err)
	}

	return tx.Commit()
}
