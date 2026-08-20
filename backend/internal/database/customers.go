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
		SELECT id, name, fullname, address, inn, COALESCE(kpp, ''),
		       COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
		       COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
		       COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
		       created_at, updated_at
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
			&customer.KPP,
			&customer.EDOIDTensor,
			&customer.EDOIDKontur,
			&customer.OKPO,
			&customer.Phone,
			&customer.Email,
			&customer.ContactPerson,
			&customer.ContactPosition,
			&customer.Comment,
			&customer.Status,
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
		SELECT id, name, fullname, address, inn, COALESCE(kpp, ''),
		       COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
		       COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
		       COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
		       created_at, updated_at
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
		&customer.KPP,
		&customer.EDOIDTensor,
		&customer.EDOIDKontur,
		&customer.OKPO,
		&customer.Phone,
		&customer.Email,
		&customer.ContactPerson,
		&customer.ContactPosition,
		&customer.Comment,
		&customer.Status,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer not found: %w", ErrNotFound)
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
		INSERT INTO customers (name, fullname, address, inn, kpp, edo_id_tensor, edo_id_kontur, okpo, phone, email, contact_person, contact_position, comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, name, fullname, address, inn, COALESCE(kpp, ''),
		          COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
		          COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
		          COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
		          created_at, updated_at
	`

	var customer models.Customer
	err = tx.QueryRowContext(
		ctx,
		query,
		req.Name,
		req.Fullname,
		req.Address,
		req.INN,
		req.KPP,
		req.EDOIDTensor,
		req.EDOIDKontur,
		req.OKPO,
		req.Phone,
		req.Email,
		req.ContactPerson,
		req.ContactPosition,
		req.Comment,
	).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Fullname,
		&customer.Address,
		&customer.INN,
		&customer.KPP,
		&customer.EDOIDTensor,
		&customer.EDOIDKontur,
		&customer.OKPO,
		&customer.Phone,
		&customer.Email,
		&customer.ContactPerson,
		&customer.ContactPosition,
		&customer.Comment,
		&customer.Status,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &customer, nil
}

// DeleteCustomer удаляет контрагента. FK-ограничения (ON DELETE RESTRICT на
// contracts.customer_id / invoices.customer_id) не дадут удалить контрагента
// со связанными договорами или счетами — вызывающий код должен показать это
// пользователю как понятную ошибку, а не удалять связанные документы молча.
func (db *DB) DeleteCustomer(ctx context.Context, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete customer: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *DB) UpdateCustomerTensorEDOID(ctx context.Context, id, edoID string) error {
	query := `
		UPDATE customers
		SET edo_id_tensor = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	result, err := db.ExecContext(ctx, query, id, edoID)
	if err != nil {
		return fmt.Errorf("failed to update customer Tensor EDO ID: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect updated customer rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("customer not found: %w", ErrNotFound)
	}
	return nil
}
