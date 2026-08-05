package database

import (
	"context"
	"database/sql"
	"fmt"

	"invoices-backend/internal/models"
)

func (db *DB) GetCustomerRedmineProject(ctx context.Context, customerID string) (*models.CustomerRedmineProjectLink, error) {
	query := `
		SELECT id, local_entity_id, external_id, COALESCE(external_identifier, ''),
		       COALESCE(external_name, ''), COALESCE(external_url, ''), last_synced_at,
		       created_at, updated_at
		FROM external_links
		WHERE local_entity_type = 'customer'
		  AND local_entity_id = $1
		  AND system = 'redmine'
		  AND external_entity_type = 'project'
		  AND is_primary = TRUE
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var link models.CustomerRedmineProjectLink
	err := db.QueryRowContext(ctx, query, customerID).Scan(
		&link.ID,
		&link.CustomerID,
		&link.RedmineProjectID,
		&link.RedmineIdentifier,
		&link.RedmineProjectName,
		&link.RedmineURL,
		&link.LastSyncedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer redmine project: %w", err)
	}
	return &link, nil
}

func (db *DB) LinkCustomerRedmineProject(ctx context.Context, customerID string, req models.LinkCustomerRedmineProjectRequest) (*models.CustomerRedmineProjectLink, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE external_links
		SET is_primary = FALSE
		WHERE local_entity_type = 'customer'
		  AND local_entity_id = $1
		  AND system = 'redmine'
		  AND external_entity_type = 'project'
	`, customerID); err != nil {
		return nil, fmt.Errorf("failed to update previous redmine project links: %w", err)
	}

	query := `
		INSERT INTO external_links (
			local_entity_type, local_entity_id, system, external_entity_type,
			external_id, external_identifier, external_name, external_url,
			is_primary, last_synced_at
		)
		VALUES ('customer', $1, 'redmine', 'project', $2, $3, $4, $5, TRUE, CURRENT_TIMESTAMP)
		RETURNING id, local_entity_id, external_id, COALESCE(external_identifier, ''),
		          COALESCE(external_name, ''), COALESCE(external_url, ''), last_synced_at,
		          created_at, updated_at
	`

	var link models.CustomerRedmineProjectLink
	err = tx.QueryRowContext(ctx, query, customerID, req.ProjectID, req.ProjectIdentifier, req.ProjectName, req.ProjectURL).Scan(
		&link.ID,
		&link.CustomerID,
		&link.RedmineProjectID,
		&link.RedmineIdentifier,
		&link.RedmineProjectName,
		&link.RedmineURL,
		&link.LastSyncedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to link customer redmine project: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit redmine project link: %w", err)
	}

	return &link, nil
}

func (db *DB) GetCustomerRedmineDocumentStatuses(ctx context.Context, customerID string) ([]models.RedmineDocumentStatus, error) {
	query := `
		SELECT document_type, document_id, COALESCE(customer_id::text, ''),
		       redmine_project_id, COALESCE(redmine_project_identifier, ''),
		       COALESCE(redmine_project_name, ''), filename, status, COALESCE(error, ''),
		       uploaded_at, created_at, updated_at
		FROM (
			SELECT DISTINCT ON (document_type, document_id)
			       document_type, document_id, customer_id, redmine_project_id,
			       redmine_project_identifier, redmine_project_name, filename,
			       status, error, uploaded_at, created_at, updated_at
			FROM redmine_document_uploads
			WHERE customer_id = $1
			ORDER BY document_type, document_id, created_at DESC
		) latest
		ORDER BY created_at DESC
	`

	rows, err := db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query redmine document statuses: %w", err)
	}
	defer rows.Close()

	var statuses []models.RedmineDocumentStatus
	for rows.Next() {
		var status models.RedmineDocumentStatus
		if err := rows.Scan(
			&status.DocumentType,
			&status.DocumentID,
			&status.CustomerID,
			&status.RedmineProjectID,
			&status.RedmineProjectIdentifier,
			&status.RedmineProjectName,
			&status.Filename,
			&status.Status,
			&status.Error,
			&status.UploadedAt,
			&status.CreatedAt,
			&status.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan redmine document status: %w", err)
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate redmine document statuses: %w", err)
	}

	return statuses, nil
}

func (db *DB) CreateAgentAuditLog(ctx context.Context, actor, operation, entityType, entityID, customerID, status, errorText string, requestPayload, responsePayload []byte) error {
	if actor == "" {
		actor = "hermes"
	}
	if len(requestPayload) == 0 {
		requestPayload = []byte("{}")
	}
	if len(responsePayload) == 0 {
		responsePayload = []byte("{}")
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO agent_audit_logs (
			actor, operation, entity_type, entity_id, customer_id,
			request_payload, response_payload, status, error
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, '')::uuid, NULLIF($5, '')::uuid,
		        $6::jsonb, $7::jsonb, $8, NULLIF($9, ''))
	`, actor, operation, entityType, entityID, customerID, requestPayload, responsePayload, status, errorText)
	if err != nil {
		return fmt.Errorf("failed to create agent audit log: %w", err)
	}
	return nil
}
