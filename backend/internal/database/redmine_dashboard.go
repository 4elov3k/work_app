package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"invoices-backend/internal/models"
)

func (db *DB) HasRedmineDashboardItems(ctx context.Context) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redmine_project_dashboard_items`).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to count redmine dashboard items: %w", err)
	}
	return count > 0, nil
}

func (db *DB) GetRedmineProjectDashboardItem(ctx context.Context, projectID string) (*models.RedmineProjectDashboardItem, error) {
	items, _, _, err := db.GetRedmineProjectDashboard(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ProjectID == projectID || item.Identifier == projectID {
			return &item, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (db *DB) GetRedmineProjectGroups(ctx context.Context) ([]models.RedmineProjectGroup, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, color, position, created_at, updated_at
		FROM redmine_project_groups
		ORDER BY position ASC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query redmine project groups: %w", err)
	}
	defer rows.Close()

	var groups []models.RedmineProjectGroup
	for rows.Next() {
		var group models.RedmineProjectGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.Color, &group.Position, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan redmine project group: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate redmine project groups: %w", err)
	}
	return groups, nil
}

func (db *DB) CreateRedmineProjectGroup(ctx context.Context, req models.CreateRedmineProjectGroupRequest) (*models.RedmineProjectGroup, error) {
	if req.Color == "" {
		req.Color = "#64748b"
	}

	query := `
		INSERT INTO redmine_project_groups (name, color, position)
		VALUES ($1, $2, COALESCE((SELECT MAX(position) + 10 FROM redmine_project_groups), 10))
		RETURNING id, name, color, position, created_at, updated_at
	`

	var group models.RedmineProjectGroup
	if err := db.QueryRowContext(ctx, query, req.Name, req.Color).Scan(
		&group.ID,
		&group.Name,
		&group.Color,
		&group.Position,
		&group.CreatedAt,
		&group.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create redmine project group: %w", err)
	}
	return &group, nil
}

func (db *DB) UpdateRedmineProjectGroup(ctx context.Context, id string, req models.UpdateRedmineProjectGroupRequest) (*models.RedmineProjectGroup, error) {
	query := `
		UPDATE redmine_project_groups
		SET name = COALESCE($2, name),
		    color = COALESCE($3, color),
		    position = COALESCE($4, position)
		WHERE id = $1
		RETURNING id, name, color, position, created_at, updated_at
	`

	var group models.RedmineProjectGroup
	if err := db.QueryRowContext(ctx, query, id, req.Name, req.Color, req.Position).Scan(
		&group.ID,
		&group.Name,
		&group.Color,
		&group.Position,
		&group.CreatedAt,
		&group.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to update redmine project group: %w", err)
	}
	return &group, nil
}

func (db *DB) UpsertRedmineProjectDashboardItem(ctx context.Context, project models.RedmineProject, baseURL string, author *models.RedmineIssueAuthor, issueID string) error {
	var managerID *string
	var managerName *string
	var inferredIssueID *string
	var inferredAt *time.Time
	if author != nil {
		id := fmt.Sprint(author.ID)
		managerID = &id
		managerName = &author.Name
		if issueID != "" {
			inferredIssueID = &issueID
		}
		now := time.Now()
		inferredAt = &now
	}

	projectURL := ""
	if baseURL != "" && project.Identifier != "" {
		projectURL = baseURL + "/projects/" + project.Identifier
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO redmine_project_dashboard_items (
			redmine_project_id, redmine_identifier, redmine_project_name, redmine_project_url,
			description, status, is_public, inferred_manager_id, inferred_manager_name,
			inferred_issue_id, inferred_at, synced_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP)
		ON CONFLICT (redmine_project_id) DO UPDATE SET
			redmine_identifier = EXCLUDED.redmine_identifier,
			redmine_project_name = EXCLUDED.redmine_project_name,
			redmine_project_url = EXCLUDED.redmine_project_url,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			is_public = EXCLUDED.is_public,
			inferred_manager_id = EXCLUDED.inferred_manager_id,
			inferred_manager_name = EXCLUDED.inferred_manager_name,
			inferred_issue_id = EXCLUDED.inferred_issue_id,
			inferred_at = EXCLUDED.inferred_at,
			synced_at = CURRENT_TIMESTAMP
	`, fmt.Sprint(project.ID), project.Identifier, project.Name, projectURL, project.Description, project.Status, project.IsPublic, managerID, managerName, inferredIssueID, inferredAt)
	if err != nil {
		return fmt.Errorf("failed to upsert redmine project dashboard item: %w", err)
	}
	return nil
}

func (db *DB) GetRedmineProjectDashboard(ctx context.Context) ([]models.RedmineProjectDashboardItem, []string, *time.Time, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.redmine_project_id, p.redmine_identifier, p.redmine_project_name,
		       COALESCE(p.redmine_project_url, ''), COALESCE(p.description, ''),
		       p.status, p.is_public, COALESCE(p.inferred_manager_id, ''),
		       COALESCE(p.inferred_manager_name, ''), COALESCE(p.manual_manager_id, ''),
		       COALESCE(p.manual_manager_name, ''),
		       COALESCE(NULLIF(p.manual_manager_id, ''), p.inferred_manager_id, ''),
		       COALESCE(NULLIF(p.manual_manager_name, ''), p.inferred_manager_name, ''),
		       COALESCE(p.manual_project_type, ''), p.urgent, COALESCE(p.urgent_reason, ''),
		       e.id::text, COALESCE(e.event_type, ''), COALESCE(e.service_type, ''),
		       COALESCE(e.title, ''), e.due_date, e.period_start, e.period_end,
		       COALESCE(e.sequence_number, 0), COALESCE(e.status, ''),
		       e.sent_at, COALESCE(e.sent_by, ''), COALESCE(e.redmine_issue_id, ''),
		       e.created_at, e.updated_at,
		       COALESCE(p.inferred_issue_id, ''),
		       p.inferred_at, COALESCE(p.group_id::text, ''),
		       COALESCE(g.name, ''), COALESCE(g.color, ''), p.group_assigned_manually,
		       p.synced_at, p.created_at, p.updated_at
		FROM redmine_project_dashboard_items p
		LEFT JOIN redmine_project_groups g ON g.id = p.group_id
		LEFT JOIN LATERAL (
			SELECT *
			FROM redmine_project_control_events
			WHERE redmine_project_id = p.redmine_project_id
			  AND status = 'planned'
			ORDER BY due_date ASC, sequence_number ASC, created_at ASC
			LIMIT 1
		) e ON TRUE
		ORDER BY COALESCE(g.position, 999999), g.name, p.redmine_project_name
	`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query redmine project dashboard: %w", err)
	}
	defer rows.Close()

	var items []models.RedmineProjectDashboardItem
	var syncedAt *time.Time
	managerSet := map[string]bool{}
	for rows.Next() {
		var item models.RedmineProjectDashboardItem
		var eventID sql.NullString
		var eventType sql.NullString
		var eventServiceType sql.NullString
		var eventTitle sql.NullString
		var eventDueDate sql.NullTime
		var eventPeriodStart sql.NullTime
		var eventPeriodEnd sql.NullTime
		var eventSequenceNumber sql.NullInt64
		var eventStatus sql.NullString
		var eventSentAt sql.NullTime
		var eventSentBy sql.NullString
		var eventRedmineIssueID sql.NullString
		var eventCreatedAt sql.NullTime
		var eventUpdatedAt sql.NullTime
		if err := rows.Scan(
			&item.ProjectID,
			&item.Identifier,
			&item.Name,
			&item.URL,
			&item.Description,
			&item.Status,
			&item.IsPublic,
			&item.InferredManagerID,
			&item.InferredManagerName,
			&item.ManualManagerID,
			&item.ManualManagerName,
			&item.EffectiveManagerID,
			&item.EffectiveManagerName,
			&item.ProjectType,
			&item.Urgent,
			&item.UrgentReason,
			&eventID,
			&eventType,
			&eventServiceType,
			&eventTitle,
			&eventDueDate,
			&eventPeriodStart,
			&eventPeriodEnd,
			&eventSequenceNumber,
			&eventStatus,
			&eventSentAt,
			&eventSentBy,
			&eventRedmineIssueID,
			&eventCreatedAt,
			&eventUpdatedAt,
			&item.InferredIssueID,
			&item.InferredAt,
			&item.GroupID,
			&item.GroupName,
			&item.GroupColor,
			&item.GroupAssignedManually,
			&item.SyncedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan redmine project dashboard item: %w", err)
		}
		if eventID.Valid {
			event := models.RedmineProjectControlEvent{
				ID:             eventID.String,
				ProjectID:      item.ProjectID,
				EventType:      eventType.String,
				ServiceType:    eventServiceType.String,
				Title:          eventTitle.String,
				SequenceNumber: int(eventSequenceNumber.Int64),
				Status:         eventStatus.String,
				SentBy:         eventSentBy.String,
				RedmineIssueID: eventRedmineIssueID.String,
			}
			if eventDueDate.Valid {
				event.DueDate = eventDueDate.Time.Format("2006-01-02")
			}
			if eventPeriodStart.Valid {
				event.PeriodStart = eventPeriodStart.Time.Format("2006-01-02")
			}
			if eventPeriodEnd.Valid {
				event.PeriodEnd = eventPeriodEnd.Time.Format("2006-01-02")
			}
			if eventSentAt.Valid {
				event.SentAt = &eventSentAt.Time
			}
			if eventCreatedAt.Valid {
				event.CreatedAt = eventCreatedAt.Time
			}
			if eventUpdatedAt.Valid {
				event.UpdatedAt = eventUpdatedAt.Time
			}
			item.NextControlEvent = &event
		}
		item.DeadlineState = deadlineState(item.Urgent, item.NextControlEvent, time.Now())
		if syncedAt == nil || item.SyncedAt.After(*syncedAt) {
			value := item.SyncedAt
			syncedAt = &value
		}
		if item.EffectiveManagerName != "" {
			managerSet[item.EffectiveManagerName] = true
		}
		if item.InferredManagerName != "" {
			managerSet[item.InferredManagerName] = true
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to iterate redmine project dashboard items: %w", err)
	}

	managers := make([]string, 0, len(managerSet))
	for manager := range managerSet {
		managers = append(managers, manager)
	}
	sort.Strings(managers)
	return items, managers, syncedAt, nil
}

func deadlineState(urgent bool, event *models.RedmineProjectControlEvent, now time.Time) string {
	if urgent {
		return "urgent"
	}
	if event == nil || event.DueDate == "" {
		return "ok"
	}
	dueDate, err := time.ParseInLocation("2006-01-02", event.DueDate, now.Location())
	if err != nil {
		return "ok"
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(dueDate.Sub(today).Hours() / 24)
	if days < 0 {
		return "burning"
	}
	if days <= 3 {
		return "due_soon"
	}
	return "ok"
}

func (db *DB) AssignRedmineProjectGroup(ctx context.Context, projectID, groupID string) error {
	var nullableGroupID sql.NullString
	if groupID != "" {
		nullableGroupID = sql.NullString{String: groupID, Valid: true}
	}

	result, err := db.ExecContext(ctx, `
		UPDATE redmine_project_dashboard_items
		SET group_id = $2::uuid,
		    group_assigned_manually = TRUE
		WHERE redmine_project_id = $1
	`, projectID, nullableGroupID)
	if err != nil {
		return fmt.Errorf("failed to assign redmine project group: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *DB) AssignRedmineProjectManager(ctx context.Context, projectID, managerID, managerName string) error {
	result, err := db.ExecContext(ctx, `
		UPDATE redmine_project_dashboard_items
		SET manual_manager_id = NULLIF($2, ''),
		    manual_manager_name = NULLIF($3, '')
		WHERE redmine_project_id = $1
	`, projectID, managerID, managerName)
	if err != nil {
		return fmt.Errorf("failed to assign redmine project manager: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *DB) UpdateRedmineProjectOperations(ctx context.Context, projectID string, req models.UpdateRedmineProjectOperationsRequest) error {
	var projectType interface{}
	if req.ProjectType != nil {
		projectType = *req.ProjectType
	}
	var urgent interface{}
	if req.Urgent != nil {
		urgent = *req.Urgent
	}
	var urgentReason interface{}
	if req.UrgentReason != nil {
		urgentReason = *req.UrgentReason
	}

	result, err := db.ExecContext(ctx, `
		UPDATE redmine_project_dashboard_items
		SET manual_project_type = CASE
		        WHEN $2::text IS NULL THEN manual_project_type
		        ELSE NULLIF($2, '')
		    END,
		    urgent = COALESCE($3, urgent),
		    urgent_reason = CASE
		        WHEN $3 = FALSE THEN NULL
		        ELSE COALESCE(NULLIF($4, ''), urgent_reason)
		    END
		WHERE redmine_project_id = $1
	`, projectID, projectType, urgent, urgentReason)
	if err != nil {
		return fmt.Errorf("failed to update redmine project operations: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *DB) GetRedmineProjectControlEvents(ctx context.Context, projectID string) ([]models.RedmineProjectControlEvent, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id::text, redmine_project_id, event_type, service_type, title,
		       due_date, period_start, period_end, sequence_number, status,
		       sent_at, COALESCE(sent_by, ''), COALESCE(redmine_issue_id, ''),
		       created_at, updated_at
		FROM redmine_project_control_events
		WHERE redmine_project_id = $1
		ORDER BY status = 'sent', due_date ASC, sequence_number ASC, created_at ASC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query redmine project control events: %w", err)
	}
	defer rows.Close()

	events := []models.RedmineProjectControlEvent{}
	for rows.Next() {
		event, err := scanRedmineProjectControlEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate redmine project control events: %w", err)
	}
	return events, nil
}

type controlEventScanner interface {
	Scan(dest ...interface{}) error
}

func scanRedmineProjectControlEvent(scanner controlEventScanner) (models.RedmineProjectControlEvent, error) {
	var event models.RedmineProjectControlEvent
	var dueDate time.Time
	var periodStart sql.NullTime
	var periodEnd sql.NullTime
	if err := scanner.Scan(
		&event.ID,
		&event.ProjectID,
		&event.EventType,
		&event.ServiceType,
		&event.Title,
		&dueDate,
		&periodStart,
		&periodEnd,
		&event.SequenceNumber,
		&event.Status,
		&event.SentAt,
		&event.SentBy,
		&event.RedmineIssueID,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return event, fmt.Errorf("failed to scan redmine project control event: %w", err)
	}
	event.DueDate = dueDate.Format("2006-01-02")
	if periodStart.Valid {
		event.PeriodStart = periodStart.Time.Format("2006-01-02")
	}
	if periodEnd.Valid {
		event.PeriodEnd = periodEnd.Time.Format("2006-01-02")
	}
	return event, nil
}

func (db *DB) GenerateRedmineProjectCycle(ctx context.Context, projectID, projectType string, reportDate time.Time) ([]models.RedmineProjectControlEvent, error) {
	if projectType == "" {
		return nil, fmt.Errorf("project type is required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin redmine project cycle transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE redmine_project_dashboard_items
		SET manual_project_type = $2
		WHERE redmine_project_id = $1
	`, projectID, projectType); err != nil {
		return nil, fmt.Errorf("failed to update project type for cycle: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM redmine_project_control_events
		WHERE redmine_project_id = $1 AND status = 'planned'
	`, projectID); err != nil {
		return nil, fmt.Errorf("failed to clear planned control events: %w", err)
	}

	if err := insertCycleEvents(ctx, tx, projectID, projectType, reportDate); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit redmine project cycle: %w", err)
	}
	return db.GetRedmineProjectControlEvents(ctx, projectID)
}

func insertCycleEvents(ctx context.Context, tx *sql.Tx, projectID, projectType string, reportDate time.Time) error {
	reportDate = dateOnly(reportDate)
	periodStart := reportDate.AddDate(0, -1, 1)
	if projectType == "ads" {
		for index, daysBefore := range []int{21, 14, 7} {
			dueDate := reportDate.AddDate(0, 0, -daysBefore)
			title := fmt.Sprintf("КС %d", index+1)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO redmine_project_control_events (
					redmine_project_id, event_type, service_type, title,
					due_date, period_start, period_end, sequence_number
				)
				VALUES ($1, 'control_cut', $2, $3, $4, $5, $6, $7)
			`, projectID, projectType, title, dueDate, periodStart, reportDate, index+1); err != nil {
				return fmt.Errorf("failed to insert ad control cut: %w", err)
			}
		}
	}

	title := "ОД"
	if projectType == "dev" {
		title = "Этап дорожной карты"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO redmine_project_control_events (
			redmine_project_id, event_type, service_type, title,
			due_date, period_start, period_end, sequence_number
		)
		VALUES ($1, 'report_date', $2, $3, $4, $5, $6, 10)
	`, projectID, projectType, title, reportDate, periodStart, reportDate); err != nil {
		return fmt.Errorf("failed to insert report date: %w", err)
	}
	return nil
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func (db *DB) MarkRedmineProjectControlEventSent(ctx context.Context, projectID, eventID, sentBy string) ([]models.RedmineProjectControlEvent, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin control event transaction: %w", err)
	}
	defer tx.Rollback()

	var eventType string
	var projectType string
	var dueDate time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE redmine_project_control_events
		SET status = 'sent', sent_at = CURRENT_TIMESTAMP, sent_by = NULLIF($3, '')
		WHERE id = $1 AND redmine_project_id = $2
		RETURNING event_type, service_type, due_date
	`, eventID, projectID, sentBy).Scan(&eventType, &projectType, &dueDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to mark control event sent: %w", err)
	}

	if eventType == "report_date" {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM redmine_project_control_events
			WHERE redmine_project_id = $1 AND status = 'planned'
		`, projectID); err != nil {
			return nil, fmt.Errorf("failed to clear planned events after report date: %w", err)
		}
		if err := insertCycleEvents(ctx, tx, projectID, projectType, dueDate.AddDate(0, 1, 0)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit control event transaction: %w", err)
	}
	return db.GetRedmineProjectControlEvents(ctx, projectID)
}

func (db *DB) DeleteRedmineProjectControlEvent(ctx context.Context, projectID, eventID string) ([]models.RedmineProjectControlEvent, error) {
	result, err := db.ExecContext(ctx, `
		DELETE FROM redmine_project_control_events
		WHERE id = $1 AND redmine_project_id = $2
	`, eventID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete redmine project control event: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	return db.GetRedmineProjectControlEvents(ctx, projectID)
}

func (db *DB) GetCustomerByRedmineProjectID(ctx context.Context, projectID string) (*models.Customer, error) {
	query := `
		SELECT c.id, c.name, c.fullname, c.address, c.inn, COALESCE(c.kpp, ''), c.created_at, c.updated_at
		FROM external_links l
		JOIN customers c ON c.id = l.local_entity_id
		WHERE l.local_entity_type = 'customer'
		  AND l.system = 'redmine'
		  AND l.external_entity_type = 'project'
		  AND l.is_primary = TRUE
		  AND (l.external_id = $1 OR l.external_identifier = $1)
		LIMIT 1
	`

	var customer models.Customer
	err := db.QueryRowContext(ctx, query, projectID).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Fullname,
		&customer.Address,
		&customer.INN,
		&customer.KPP,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

func (db *DB) GetLocalDocumentsForRedmineProject(ctx context.Context, projectID string) (string, string, []models.RedmineProjectLocalDocument, error) {
	customer, err := db.GetCustomerByRedmineProjectID(ctx, projectID)
	if err == sql.ErrNoRows {
		return "", "", []models.RedmineProjectLocalDocument{}, nil
	}
	if err != nil {
		return "", "", nil, err
	}

	var docs []models.RedmineProjectLocalDocument

	invoices, _, err := db.GetInvoices(ctx, customer.ID, "", nil, 1, 1000)
	if err != nil {
		return "", "", nil, err
	}
	for _, invoice := range invoices {
		status, uploadedAt := db.latestRedmineDocumentUploadStatus(ctx, "invoice", invoice.ID)
		docs = append(docs, models.RedmineProjectLocalDocument{
			ID:             invoice.ID,
			Type:           "invoice",
			Number:         invoice.Number,
			Date:           invoice.Date,
			Status:         invoice.Status,
			TotalAmount:    invoice.TotalAmount,
			ContractNumber: invoice.ContractNumber,
			UploadedStatus: status,
			UploadedAt:     uploadedAt,
			URL:            "/" + customer.ID + "/" + invoice.ID,
		})
	}

	acts, _, err := db.GetActs(ctx, customer.ID, "", nil, 1, 1000)
	if err != nil {
		return "", "", nil, err
	}
	for _, act := range acts {
		status, uploadedAt := db.latestRedmineDocumentUploadStatus(ctx, "act", act.ID)
		docs = append(docs, models.RedmineProjectLocalDocument{
			ID:             act.ID,
			Type:           "act",
			Number:         act.Number,
			Date:           act.Date,
			Status:         act.Status,
			TotalAmount:    act.TotalAmount,
			ContractNumber: act.ContractNumber,
			UploadedStatus: status,
			UploadedAt:     uploadedAt,
			URL:            "/" + customer.ID + "/acts/" + act.ID,
		})
	}

	return customer.ID, customer.Name, docs, nil
}

func (db *DB) latestRedmineDocumentUploadStatus(ctx context.Context, documentType, documentID string) (string, string) {
	var status string
	var uploadedAt sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT status, uploaded_at
		FROM redmine_document_uploads
		WHERE document_type = $1 AND document_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, documentType, documentID).Scan(&status, &uploadedAt)
	if err != nil {
		return "", ""
	}
	if uploadedAt.Valid {
		return status, uploadedAt.Time.Format(time.RFC3339)
	}
	return status, ""
}

func (db *DB) RecordRedmineDocumentUpload(ctx context.Context, documentType, documentID, customerID, projectID, projectIdentifier, projectName, fileID, filename, status, errorText string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO redmine_document_uploads (
			document_type, document_id, customer_id, redmine_project_id,
			redmine_project_identifier, redmine_project_name, redmine_file_id,
			filename, content_type, status, error, uploaded_at
		)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, NULLIF($7, ''),
		        $8, 'application/pdf', $9, NULLIF($10, ''),
		        CASE WHEN $9 = 'uploaded' THEN CURRENT_TIMESTAMP ELSE NULL END)
	`, documentType, documentID, customerID, projectID, projectIdentifier, projectName, fileID, filename, status, errorText)
	if err != nil {
		return fmt.Errorf("failed to record redmine document upload: %w", err)
	}
	return nil
}
