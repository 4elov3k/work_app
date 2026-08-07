package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"invoices-backend/internal/models"
)

// CreateCallerReportParams описывает параметры для сохранения отчёта по звонарю
type CreateCallerReportParams struct {
	CallerID    string
	Period      string
	PeriodStart string
	PeriodEnd   string
	SummaryText string
	MetricsJSON json.RawMessage
}

// CreateCallerReport сохраняет сгенерированный Hermes отчёт по звонарю за период
func (db *DB) CreateCallerReport(ctx context.Context, params CreateCallerReportParams) (*models.CallerReport, error) {
	query := `
		INSERT INTO caller_reports (caller_id, period, period_start, period_end, summary_text, metrics_json)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, caller_id, period, period_start, period_end, summary_text, metrics_json, requested_at
	`
	var r models.CallerReport
	var metrics []byte
	err := db.QueryRowContext(ctx, query,
		params.CallerID, params.Period, params.PeriodStart, params.PeriodEnd, params.SummaryText, []byte(params.MetricsJSON),
	).Scan(&r.ID, &r.CallerID, &r.Period, &r.PeriodStart, &r.PeriodEnd, &r.SummaryText, &metrics, &r.RequestedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create caller report: %w", err)
	}
	if len(metrics) > 0 {
		r.MetricsJSON = json.RawMessage(metrics)
	}
	return &r, nil
}

// GetLatestCallerReport returns the most recent report for exactly this
// caller+period (nil if none exists yet) — lets a caller re-use an
// already-generated report instead of re-paying the LLM cost for the same
// period every time.
func (db *DB) GetLatestCallerReport(ctx context.Context, callerID, periodStart, periodEnd string) (*models.CallerReport, error) {
	query := `
		SELECT id, caller_id, period, period_start, period_end, summary_text, metrics_json, requested_at
		FROM caller_reports
		WHERE caller_id = $1 AND period_start = $2 AND period_end = $3
		ORDER BY requested_at DESC
		LIMIT 1
	`
	var r models.CallerReport
	var metrics []byte
	err := db.QueryRowContext(ctx, query, callerID, periodStart, periodEnd).Scan(
		&r.ID, &r.CallerID, &r.Period, &r.PeriodStart, &r.PeriodEnd, &r.SummaryText, &metrics, &r.RequestedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest caller report: %w", err)
	}
	if len(metrics) > 0 {
		r.MetricsJSON = json.RawMessage(metrics)
	}
	return &r, nil
}

// ListCallerReports returns a caller's past reports, most recent first —
// so previously generated analyses stay visible/reachable instead of only
// existing as rows nobody ever reads back.
func (db *DB) ListCallerReports(ctx context.Context, callerID string, limit int) ([]models.CallerReport, error) {
	query := `
		SELECT id, caller_id, period, period_start, period_end, summary_text, metrics_json, requested_at
		FROM caller_reports
		WHERE caller_id = $1
		ORDER BY requested_at DESC
		LIMIT $2
	`
	rows, err := db.QueryContext(ctx, query, callerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list caller reports: %w", err)
	}
	defer rows.Close()

	var reports []models.CallerReport
	for rows.Next() {
		var r models.CallerReport
		var metrics []byte
		if err := rows.Scan(&r.ID, &r.CallerID, &r.Period, &r.PeriodStart, &r.PeriodEnd, &r.SummaryText, &metrics, &r.RequestedAt); err != nil {
			return nil, fmt.Errorf("failed to scan caller report: %w", err)
		}
		if len(metrics) > 0 {
			r.MetricsJSON = json.RawMessage(metrics)
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating caller reports: %w", err)
	}
	return reports, nil
}
