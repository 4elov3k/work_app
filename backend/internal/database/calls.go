package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"

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

// GetCallByID returns a single call by its internal ID, e.g. for a manual
// "(re)transcribe this call" trigger from the UI.
func (db *DB) GetCallByID(ctx context.Context, id string) (*models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json, created_at, updated_at
		FROM calls
		WHERE id = $1
	`
	var c models.Call
	var analytics []byte
	err := db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
		&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
		&c.TranscriptText, &analytics, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("call not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get call: %w", err)
	}
	if len(analytics) > 0 {
		c.AnalyticsJSON = json.RawMessage(analytics)
	}
	return &c, nil
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
// CountCallsByCallerPeriod returns, for every caller with at least one call
// in [from, to), how many synced calls they have — one query for all
// callers, so rendering the caller list doesn't need N per-card requests.
func (db *DB) CountCallsByCallerPeriod(ctx context.Context, from, to time.Time) (map[string]int, error) {
	query := `
		SELECT caller_id, count(*)
		FROM calls
		WHERE caller_id IS NOT NULL AND started_at >= $1 AND started_at < $2
		GROUP BY caller_id
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count calls: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var callerID string
		var count int
		if err := rows.Scan(&callerID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan call count: %w", err)
		}
		counts[callerID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating call counts: %w", err)
	}
	return counts, nil
}

// CountCallsByCallerAndStatus returns, for every caller with calls in
// [from, to), a breakdown of how many calls they have per transcript_status
// (done/failed/no_recording/transcribing/pending) — one query, for the
// caller-list "full statistics" view instead of raw counts hiding whether
// transcription actually succeeded.
func (db *DB) CountCallsByCallerAndStatus(ctx context.Context, from, to time.Time) (map[string]map[string]int, error) {
	query := `
		SELECT caller_id, transcript_status, count(*)
		FROM calls
		WHERE caller_id IS NOT NULL AND started_at >= $1 AND started_at < $2
		GROUP BY caller_id, transcript_status
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count calls by status: %w", err)
	}
	defer rows.Close()

	counts := map[string]map[string]int{}
	for rows.Next() {
		var callerID, status string
		var count int
		if err := rows.Scan(&callerID, &status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan call status count: %w", err)
		}
		if counts[callerID] == nil {
			counts[callerID] = map[string]int{}
		}
		counts[callerID][status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating call status counts: %w", err)
	}
	return counts, nil
}

// ListCallsByStatusPeriod returns all calls (any caller) in [from, to)
// whose transcript_status is one of the given values — for a bulk "retry
// everything that didn't finish processing" job.
func (db *DB) ListCallsByStatusPeriod(ctx context.Context, statuses []string, from, to time.Time) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json, created_at, updated_at
		FROM calls
		WHERE transcript_status = ANY($1) AND started_at >= $2 AND started_at < $3
		ORDER BY started_at
	`
	rows, err := db.QueryContext(ctx, query, pq.Array(statuses), from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls by status: %w", err)
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
