package database

import (
	"context"
	"database/sql"
	"fmt"

	"invoices-backend/internal/models"
)

// GetContracts возвращает список договоров с опциональной фильтрацией по контрагенту
func (db *DB) GetContracts(ctx context.Context, customerID string, page, perPage int) ([]models.Contract, int, error) {
	var contracts []models.Contract
	var total int

	countQuery := "SELECT COUNT(*) FROM contracts"
	var args []interface{}
	argCount := 1

	if customerID != "" {
		countQuery += " WHERE customer_id = $" + fmt.Sprint(argCount)
		args = append(args, customerID)
		argCount++
	}

	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count contracts: %w", err)
	}

	query := `
		SELECT id, customer_id, number, currency, status, topic,
		       COALESCE(start_date::text, '') AS start_date,
		       COALESCE(end_date::text, '') AS end_date,
		       created_at, updated_at
		FROM contracts
	`

	args = nil
	argCount = 1
	if customerID != "" {
		query += " WHERE customer_id = $" + fmt.Sprint(argCount)
		args = append(args, customerID)
		argCount++
	}

	query += " ORDER BY created_at DESC"
	if perPage > 0 {
		offset := (page - 1) * perPage
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
		args = append(args, perPage, offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query contracts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c models.Contract
		if err := rows.Scan(
			&c.ID,
			&c.CustomerID,
			&c.Number,
			&c.Currency,
			&c.Status,
			&c.Topic,
			&c.StartDate,
			&c.EndDate,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan contract: %w", err)
		}
		contracts = append(contracts, c)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating contracts: %w", err)
	}

	return contracts, total, nil
}

// GetContractByID возвращает договор по ID
func (db *DB) GetContractByID(ctx context.Context, id string) (*models.Contract, error) {
	query := `
		SELECT id, customer_id, number, currency, status, topic,
		       COALESCE(start_date::text, '') AS start_date,
		       COALESCE(end_date::text, '') AS end_date,
		       created_at, updated_at
		FROM contracts
		WHERE id = $1
	`
	var c models.Contract
	err := db.QueryRowContext(ctx, query, id).Scan(
		&c.ID,
		&c.CustomerID,
		&c.Number,
		&c.Currency,
		&c.Status,
		&c.Topic,
		&c.StartDate,
		&c.EndDate,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("contract not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}
	return &c, nil
}

// GetContractByCustomerAndNumber возвращает договор по customer_id + number
func (db *DB) GetContractByCustomerAndNumber(ctx context.Context, customerID, number string) (*models.Contract, error) {
	query := `
		SELECT id, customer_id, number, currency, status, topic,
		       COALESCE(start_date::text, '') AS start_date,
		       COALESCE(end_date::text, '') AS end_date,
		       created_at, updated_at
		FROM contracts
		WHERE customer_id = $1 AND number = $2
	`
	var c models.Contract
	err := db.QueryRowContext(ctx, query, customerID, number).Scan(
		&c.ID,
		&c.CustomerID,
		&c.Number,
		&c.Currency,
		&c.Status,
		&c.Topic,
		&c.StartDate,
		&c.EndDate,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("contract not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}
	return &c, nil
}

// CreateContract создает договор
func (db *DB) CreateContract(ctx context.Context, req models.CreateContractRequest) (*models.Contract, error) {
	query := `
		INSERT INTO contracts (customer_id, number, currency, status, topic, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::date, NULLIF($7, '')::date)
		RETURNING id, customer_id, number, currency, status, topic,
		          COALESCE(start_date::text, '') AS start_date,
		          COALESCE(end_date::text, '') AS end_date,
		          created_at, updated_at
	`
	var c models.Contract
	err := db.QueryRowContext(ctx, query, req.CustomerID, req.Number, req.Currency, req.Status, req.Topic, req.StartDate, req.EndDate).Scan(
		&c.ID,
		&c.CustomerID,
		&c.Number,
		&c.Currency,
		&c.Status,
		&c.Topic,
		&c.StartDate,
		&c.EndDate,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract: %w", err)
	}
	return &c, nil
}

// CreateDefaultContractTx создает договор по умолчанию в рамках транзакции
func CreateDefaultContractTx(ctx context.Context, tx *sql.Tx, customerID string) error {
	query := `
		INSERT INTO contracts (customer_id, number, currency, status, topic, start_date)
		VALUES ($1, 'Основной', 'RUB', 'active', 'Продвижение сео', CURRENT_DATE)
		ON CONFLICT (customer_id, number) DO NOTHING
	`
	if _, err := tx.ExecContext(ctx, query, customerID); err != nil {
		return fmt.Errorf("failed to create default contract: %w", err)
	}
	return nil
}

// DeleteContract удаляет договор по ID
func (db *DB) DeleteContract(ctx context.Context, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM contracts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete contract: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetNextContractNumber возвращает следующий номер договора для клиента
func (db *DB) GetNextContractNumber(ctx context.Context, customerID string) (int64, error) {
	query := `
		SELECT COALESCE(MAX(number::bigint), 699) + 1
		FROM contracts
		WHERE customer_id = $1
		  AND number ~ '^[0-9]+$'
	`
	var next int64
	if err := db.QueryRowContext(ctx, query, customerID).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to get next contract number: %w", err)
	}
	return next, nil
}
