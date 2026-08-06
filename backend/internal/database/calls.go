package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"invoices-backend/internal/models"
)

// InsertCall inserts a new CDR row. Returns inserted=false when pbx_uuid
// already exists (ON CONFLICT DO NOTHING), so re-syncing an overlapping
// period is a safe no-op instead of re-triggering transcription for calls
// already processed.
func (db *DB) InsertCall(ctx context.Context, call models.Call) (*models.Call, bool, error) {
	query := `
		INSERT INTO calls (pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec, hangup_cause, transcript_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
		ON CONFLICT (pbx_uuid) DO NOTHING
		RETURNING id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec, hangup_cause, transcript_status, created_at, updated_at
	`
	var c models.Call
	err := db.QueryRowContext(ctx, query,
		call.PBXUUID, call.CallerID, call.Direction, call.CounterpartyNumber,
		call.StartedAt, call.DurationSec, call.TalkTimeSec, call.HangupCause,
	).Scan(
		&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
		&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to insert call: %w", err)
	}
	return &c, true, nil
}

// SetCallTranscriptStatus updates only the processing status (e.g. while a
// transcription is in flight, or when it failed / no recording exists).
func (db *DB) SetCallTranscriptStatus(ctx context.Context, id, status string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE calls SET transcript_status = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, status,
	)
	if err != nil {
		return fmt.Errorf("failed to update transcript status: %w", err)
	}
	return nil
}

// SetCallTranscript stores the Whisper transcript and marks the call done.
func (db *DB) SetCallTranscript(ctx context.Context, id, text string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE calls SET transcript_text = $2, transcript_status = 'done', updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, text,
	)
	if err != nil {
		return fmt.Errorf("failed to update transcript: %w", err)
	}
	return nil
}

// SetCallAnalytics stores Hermes' per-call ChatGPT classification.
func (db *DB) SetCallAnalytics(ctx context.Context, id string, analytics json.RawMessage) error {
	_, err := db.ExecContext(ctx,
		`UPDATE calls SET analytics_json = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, []byte(analytics),
	)
	if err != nil {
		return fmt.Errorf("failed to update analytics: %w", err)
	}
	return nil
}

// ListCallsByCallerPeriod returns a caller's calls with started_at in [from, to).
func (db *DB) ListCallsByCallerPeriod(ctx context.Context, callerID string, from, to time.Time) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json, created_at, updated_at
		FROM calls
		WHERE caller_id = $1 AND started_at >= $2 AND started_at < $3
		ORDER BY started_at
	`
	rows, err := db.QueryContext(ctx, query, callerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls: %w", err)
	}
	defer rows.Close()

	var calls []models.Call
	for rows.Next() {
		var c models.Call
		var analytics []byte
		if err := rows.Scan(
			&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
			&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
			&c.TranscriptText, &analytics, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		if len(analytics) > 0 {
			c.AnalyticsJSON = json.RawMessage(analytics)
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}
	return calls, nil
}
