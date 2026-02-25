package database

import (
	"context"
	"database/sql"
	"fmt"

	"invoices-backend/internal/models"
)

// GetCustomers возвращает список всех контрагентов с опциональным поиском
func (db *DB) GetCustomers(ctx context.Context, search string, page, perPage int) ([]models.Customer, int, error) {
	var customers []models.Customer
	var total int

	// Подсчет общего количества
	countQuery := "SELECT COUNT(*) FROM customers"
	var args []interface{}
	argCount := 1

	if search != "" {
		countQuery += " WHERE name ILIKE $" + fmt.Sprint(argCount)
		args = append(args, "%"+search+"%")
		argCount++
	}

	err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count customers: %w", err)
	}

	// Получение данных с пагинацией
	query := `
		SELECT id, name, fullname, address, inn, created_at, updated_at 
		FROM customers
	`

	args = nil
	argCount = 1

	if search != "" {
		query += " WHERE name ILIKE $" + fmt.Sprint(argCount)
		args = append(args, "%"+search+"%")
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
		return nil, 0, fmt.Errorf("failed to query customers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var customer models.Customer
		err := rows.Scan(
			&customer.ID,
			&customer.Name,
			&customer.Fullname,
			&customer.Address,
			&customer.INN,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan customer: %w", err)
		}
		customers = append(customers, customer)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating customers: %w", err)
	}

	return customers, total, nil
}

// GetCustomerByID возвращает контрагента по ID
func (db *DB) GetCustomerByID(ctx context.Context, id string) (*models.Customer, error) {
	query := `
		SELECT id, name, fullname, address, inn, created_at, updated_at 
		FROM customers 
		WHERE id = $1
	`

	var customer models.Customer
	err := db.QueryRowContext(ctx, query, id).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Fullname,
		&customer.Address,
		&customer.INN,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return &customer, nil
}

// CreateCustomer создает нового контрагента
func (db *DB) CreateCustomer(ctx context.Context, req models.CreateCustomerRequest) (*models.Customer, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO customers (name, fullname, address, inn)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, fullname, address, inn, created_at, updated_at
	`

	var customer models.Customer
	err = tx.QueryRowContext(ctx, query, req.Name, req.Fullname, req.Address, req.INN).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Fullname,
		&customer.Address,
		&customer.INN,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	if err := CreateDefaultContractTx(ctx, tx, customer.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &customer, nil
}
