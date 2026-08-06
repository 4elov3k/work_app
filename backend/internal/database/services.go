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
	unit := req.Unit
	if unit == "" {
		unit = "услуга"
	}
	query := `
		INSERT INTO services (name, price, unit, section, price_per_hour, hours_per_unit)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), NULLIF($6, 0))
		RETURNING id, name, price, unit, section, COALESCE(price_per_hour, 0), COALESCE(hours_per_unit, 0), archived, created_at, updated_at
	`

	var service models.Service
	err := db.QueryRowContext(ctx, query, req.Name, req.Price, unit, req.Section, req.PricePerHour, req.HoursPerUnit).Scan(
		&service.ID,
		&service.Name,
		&service.Price,
		&service.Unit,
		&service.Section,
		&service.PricePerHour,
		&service.HoursPerUnit,
		&service.Archived,
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
		SELECT id, name, price, unit, section, COALESCE(price_per_hour, 0), COALESCE(hours_per_unit, 0), archived, created_at, updated_at
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
			&service.Unit,
			&service.Section,
			&service.PricePerHour,
			&service.HoursPerUnit,
			&service.Archived,
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
		SELECT id, name, price, unit, section, COALESCE(price_per_hour, 0), COALESCE(hours_per_unit, 0), archived, created_at, updated_at
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
			&service.Unit,
			&service.Section,
			&service.PricePerHour,
			&service.HoursPerUnit,
			&service.Archived,
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

// GetServiceCatalog возвращает каталожные позиции услуг (с непустым section),
// сгруппированные по разделам в порядке их естественного появления в прайсе.
func (db *DB) GetServiceCatalog(ctx context.Context) ([]models.ServiceCatalogSection, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, price, unit, section, COALESCE(price_per_hour, 0), COALESCE(hours_per_unit, 0), archived, created_at, updated_at
		FROM services
		WHERE section <> '' AND NOT archived
		ORDER BY section, created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query service catalog: %w", err)
	}
	defer rows.Close()

	order := []string{}
	bySection := map[string][]models.Service{}
	for rows.Next() {
		var service models.Service
		if err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Price,
			&service.Unit,
			&service.Section,
			&service.PricePerHour,
			&service.HoursPerUnit,
			&service.Archived,
			&service.CreatedAt,
			&service.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan service catalog item: %w", err)
		}
		if _, ok := bySection[service.Section]; !ok {
			order = append(order, service.Section)
		}
		bySection[service.Section] = append(bySection[service.Section], service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating service catalog: %w", err)
	}

	sections := make([]models.ServiceCatalogSection, 0, len(order))
	for _, section := range order {
		sections = append(sections, models.ServiceCatalogSection{Section: section, Items: bySection[section]})
	}
	return sections, nil
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
