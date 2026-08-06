package database

import (
	"context"
	"database/sql"
	"fmt"

	"invoices-backend/internal/models"
)

// UpsertCaller creates or updates a caller by their OnlinePBX extension.
// Called on every sync from the live user/get.json list, so a renamed or
// disabled agent is reflected without any manual admin step.
func (db *DB) UpsertCaller(ctx context.Context, extension, name string, active bool) (*models.Caller, error) {
	query := `
		INSERT INTO callers (pbx_extension, name, active)
		VALUES ($1, $2, $3)
		ON CONFLICT (pbx_extension) DO UPDATE
		SET name = EXCLUDED.name, active = EXCLUDED.active, updated_at = CURRENT_TIMESTAMP
		RETURNING id, pbx_extension, name, active, created_at, updated_at
	`
	var caller models.Caller
	err := db.QueryRowContext(ctx, query, extension, name, active).Scan(
		&caller.ID, &caller.PBXExtension, &caller.Name, &caller.Active, &caller.CreatedAt, &caller.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert caller: %w", err)
	}
	return &caller, nil
}

// ListCallers возвращает всех звонарей
func (db *DB) ListCallers(ctx context.Context) ([]models.Caller, error) {
	query := `SELECT id, pbx_extension, name, active, created_at, updated_at FROM callers ORDER BY name`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query callers: %w", err)
	}
	defer rows.Close()

	var callers []models.Caller
	for rows.Next() {
		var c models.Caller
		if err := rows.Scan(&c.ID, &c.PBXExtension, &c.Name, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan caller: %w", err)
		}
		callers = append(callers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating callers: %w", err)
	}
	return callers, nil
}

// GetCallerByExtension возвращает звонаря по внутреннему номеру, либо nil,
// если номер не найден (например, это техническая линия, а не звонарь).
func (db *DB) GetCallerByExtension(ctx context.Context, extension string) (*models.Caller, error) {
	query := `SELECT id, pbx_extension, name, active, created_at, updated_at FROM callers WHERE pbx_extension = $1`
	var c models.Caller
	err := db.QueryRowContext(ctx, query, extension).Scan(
		&c.ID, &c.PBXExtension, &c.Name, &c.Active, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get caller by extension: %w", err)
	}
	return &c, nil
}

// GetCallerByID возвращает звонаря по ID
func (db *DB) GetCallerByID(ctx context.Context, id string) (*models.Caller, error) {
	query := `SELECT id, pbx_extension, name, active, created_at, updated_at FROM callers WHERE id = $1`
	var c models.Caller
	err := db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.PBXExtension, &c.Name, &c.Active, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("caller not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get caller: %w", err)
	}
	return &c, nil
}
