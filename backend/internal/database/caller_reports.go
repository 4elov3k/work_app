package database

import (
	"context"
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
