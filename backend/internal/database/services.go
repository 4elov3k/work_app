package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"invoices-backend/internal/models"
)

// CreateService создает новую услугу
func (db *DB) CreateService(ctx context.Context, req models.CreateServiceRequest) (*models.Service, error) {
	query := `
		INSERT INTO services (name, price)
		VALUES ($1, $2)
		RETURNING id, name, price, created_at, updated_at
	`

	var service models.Service
	err := db.QueryRowContext(ctx, query, req.Name, req.Price).Scan(
		&service.ID,
		&service.Name,
		&service.Price,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return &service, nil
}

// GetServicesByIDs возвращает услуги по массиву ID
func (db *DB) GetServicesByIDs(ctx context.Context, ids []string) ([]models.Service, error) {
	if len(ids) == 0 {
		return []models.Service{}, nil
	}

	// Строим placeholders для IN clause: $1, $2, $3...
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
	`, strings.Join(placeholders, ", "))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query services: %w", err)
	}
	defer rows.Close()

	var services []models.Service
	for rows.Next() {
		var service models.Service
		err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Price,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan service: %w", err)
		}
		services = append(services, service)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}

	return services, nil
}

// GetServices возвращает список услуг
func (db *DB) GetServices(ctx context.Context, page, perPage int) ([]models.Service, int, error) {
	var services []models.Service
	var total int

	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM services").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count services: %w", err)
	}

	query := `
		SELECT id, name, price, created_at, updated_at
		FROM services
		ORDER BY created_at DESC
	`
	args := []interface{}{}
	if perPage > 0 {
		offset := (page - 1) * perPage
		query += fmt.Sprintf(" LIMIT $1 OFFSET $2")
		args = append(args, perPage, offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query services: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var service models.Service
		if err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Price,
			&service.CreatedAt,
			&service.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan service: %w", err)
		}
		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating services: %w", err)
	}

	return services, total, nil
}

// DeleteService удаляет услугу по ID
func (db *DB) DeleteService(ctx context.Context, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
