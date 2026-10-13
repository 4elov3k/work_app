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
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json,
		       COALESCE(engine, ''), transcribed_at, COALESCE(error_kind, ''), COALESCE(last_error, ''), created_at, updated_at
		FROM calls
		WHERE id = $1
	`
	var c models.Call
	var analytics []byte
	var transcribedAt sql.NullTime
	err := db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
		&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
		&c.TranscriptText, &analytics, &c.Engine, &transcribedAt, &c.ErrorKind, &c.LastError, &c.CreatedAt, &c.UpdatedAt,
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
	if transcribedAt.Valid {
		c.TranscribedAt = &transcribedAt.Time
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

// SetCallTranscriptError records a failed transcription attempt with why —
// error_kind is the closed set used for grouping in GetErrorBreakdown
// ("no_recording"/"download_failed"/"transcribe_failed"), lastError is the
// raw error text for the call's own detail view. Separate from
// SetCallTranscriptStatus because a bare status transition (e.g. "pending"
// -> "transcribing") isn't an error and shouldn't touch these fields.
func (db *DB) SetCallTranscriptError(ctx context.Context, id, status, errorKind, lastError string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE calls SET transcript_status = $2, error_kind = $3, last_error = $4, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, status, errorKind, lastError,
	)
	if err != nil {
		return fmt.Errorf("failed to record transcript error: %w", err)
	}
	return nil
}

// HallucinationSuspectedErrorKind flags a transcript whose length is
// implausible for the call's actual duration (see
// zvonari.Service.transcribeOnly, which computes chars-per-second right
// after a successful Transcribe() call) — almost certainly a fabricated
// LLM transcription (observed on TRANSCRIBE_FORMAT=openrouter, and on the
// August 2026 GPU-slot backfill that temporarily ran through an LLM
// sidecar) rather than a real transcript of near-silence/noise. Unlike the
// other error_kind values, this is set alongside transcript_status='done'
// (not 'failed') and the text IS kept — the call needs a human to look at
// it, not a retry, and the text itself is the evidence.
const HallucinationSuspectedErrorKind = "hallucination_suspected"

// SetCallTranscript stores the transcript and marks the call done,
// recording which engine (cpu/gpu/openrouter) actually produced it and
// when — see transcribe.Client.Transcribe, which reports back whichever
// backend/format answered. Clears any previous error_kind/last_error,
// UNLESS hallucinationSuspected is true, in which case error_kind is set to
// HallucinationSuspectedErrorKind instead of being cleared — see its doc
// comment. A successful (re)transcription otherwise supersedes whatever
// went wrong last time.
// SetCallTranscript saves the transcript and, when engine == "openrouter",
// the fraud precheck signal computed in the same OpenRouter call (see
// transcribe.Client.transcribeOpenRouter) — one write instead of two, since
// they're now always produced together. fraudCheck == "" means no signal was
// computed at all (GPU/CPU path, which has no LLM to ask) — that's not the
// same as fraudSuspected == false (a real "checked, not fraud" verdict), so
// fraud_precheck_check stores the empty string as NULL to keep that
// distinction visible in the DB rather than collapsing both into "false".
// fraudSuspected only ever sets fraud_review_status to 'pending' — never
// clears an existing review a human already made, and never sets 'confirmed'/
// 'rejected' itself, since only a human does that (see fraud_reviewed_at).
func (db *DB) SetCallTranscript(ctx context.Context, id, text, engine string, hallucinationSuspected bool, fraudSuspected bool, fraudCheck, fraudQuote, fraudPosition string) error {
	var errorKind any
	if hallucinationSuspected {
		errorKind = HallucinationSuspectedErrorKind
	}
	var suspected, check, quote, position, reviewStatus any
	if fraudCheck != "" {
		// A real check ran — record its verdict, even when that verdict is
		// "not suspected" (fraudSuspected == false is meaningful here, not a
		// zero-value stand-in for "unknown" the way it would be below).
		suspected = fraudSuspected
		check = fraudCheck
	}
	if fraudQuote != "" {
		quote = fraudQuote
	}
	if fraudPosition != "" {
		position = fraudPosition
	}
	if fraudSuspected {
		reviewStatus = "pending"
	}
	_, err := db.ExecContext(ctx,
		`UPDATE calls SET transcript_text = $2, transcript_status = 'done', engine = $3, transcribed_at = CURRENT_TIMESTAMP,
		       error_kind = $4, last_error = NULL, updated_at = CURRENT_TIMESTAMP,
		       fraud_precheck_suspected = $5, fraud_precheck_check = $6, fraud_precheck_quote = $7, fraud_precheck_position = $8,
		       fraud_review_status = COALESCE(fraud_review_status, $9)
		 WHERE id = $1`,
		id, text, engine, errorKind, suspected, check, quote, position, reviewStatus,
	)
	if err != nil {
		return fmt.Errorf("failed to update transcript: %w", err)
	}
	return nil
}

// GetErrorBreakdown counts calls in [from, to) by error_kind, across every
// caller — the "сводка ошибок по причинам" summary under the sync block
// (задача 6, zvonari-improvements.md), so a red status line becomes actual
// reasons instead of one undifferentiated "ошибка".
func (db *DB) GetErrorBreakdown(ctx context.Context, from, to time.Time) (map[string]int, error) {
	query := `
		SELECT error_kind, count(*)
		FROM calls
		WHERE error_kind IS NOT NULL AND started_at >= $1 AND started_at < $2
		GROUP BY error_kind
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count errors by kind: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return nil, fmt.Errorf("failed to scan error kind count: %w", err)
		}
		counts[kind] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating error kind counts: %w", err)
	}
	return counts, nil
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

// CountOutcomesByCaller returns, for every caller with calls in [from, to),
// a breakdown of their Hermes outcome classification (analytics_json.outcome)
// — one query across all callers, so the caller table can sort/highlight by
// script-compliance (e.g. how many reached "Скрипт пройден до шага 6" vs
// "Срыв на шаге 1") without an N+1 fetch per caller. `outcome` always holds
// one of the 13 closed-list values (§11 of the IQ-200 v1.2 regulation) once
// analyzed, including for technical/fraud-suspected calls — fraud_suspected
// is a separate boolean axis (see CountFraudSuspectedByCaller), not an
// outcome value. Calls without an outcome yet (not analyzed, analysis
// failed, or still in the pre-rubric legacy {category, outcome} shape)
// bucket under "не проанализировано", matching CallDistribution's
// client-facing label for a single caller.
func (db *DB) CountOutcomesByCaller(ctx context.Context, from, to time.Time) (map[string]map[string]int, error) {
	query := `
		SELECT caller_id, COALESCE(analytics_json->>'outcome', 'не проанализировано'), count(*)
		FROM calls
		WHERE caller_id IS NOT NULL AND started_at >= $1 AND started_at < $2
		GROUP BY caller_id, COALESCE(analytics_json->>'outcome', 'не проанализировано')
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count outcomes by caller: %w", err)
	}
	defer rows.Close()

	counts := map[string]map[string]int{}
	for rows.Next() {
		var callerID, outcome string
		var count int
		if err := rows.Scan(&callerID, &outcome, &count); err != nil {
			return nil, fmt.Errorf("failed to scan outcome count: %w", err)
		}
		if counts[callerID] == nil {
			counts[callerID] = map[string]int{}
		}
		counts[callerID][outcome] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outcome counts: %w", err)
	}
	return counts, nil
}

// CriteriaScore is one caller's aggregate over the 4-criteria rubric
// (2026-09-29 onward) for a period — MetTrue/Applicable across all 4
// criteria of all their scored calls combined, not an average of per-call
// fractions (a caller with one 4/4 call and one 0/4 call reads as 4/8, not
// as "50% on average" — the two aren't the same number once call counts
// differ, and the combined-count version is the one that doesn't silently
// let a single short call swing the number as much as a long one).
type CriteriaScore struct {
	MetTrue    int
	Applicable int
}

// CountCriteriaScoresByCaller returns, for every caller with at least one
// 4-criteria-scored call in [from, to), their combined MetTrue/Applicable —
// replaces CountOutcomesByCaller's step-funnel columns ("Скрипт до конца"/
// "Срыв на шаге 1") on the dashboard now that there's no script/steps left
// to fail at (2026-09-29, management retired the step rubric — see
// hermes/services/call_analytics_server.py's module docstring). Calls
// analyzed under the old {steps: ..., outcome: ...} shape have no
// "criteria" key and are silently excluded here, same as
// ListCallsNeedingAnalysis already excludes anything with the new
// "criteria" key from needing re-analysis — the two schemas never mix
// within one call's row, so there's no double-counting either way. Uses
// jsonb_each to iterate the 4 criteria keys per call without hardcoding
// their names in SQL.
func (db *DB) CountCriteriaScoresByCaller(ctx context.Context, from, to time.Time) (map[string]CriteriaScore, error) {
	query := `
		SELECT c.caller_id,
		       count(*) FILTER (WHERE (kv.value->>'met')::boolean IS TRUE) AS met_true,
		       count(*) FILTER (WHERE kv.value->>'met' IS NOT NULL) AS applicable
		FROM calls c, jsonb_each(c.analytics_json->'criteria') AS kv
		WHERE c.caller_id IS NOT NULL AND c.analytics_json ? 'criteria'
		  AND c.started_at >= $1 AND c.started_at < $2
		GROUP BY c.caller_id
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count criteria scores by caller: %w", err)
	}
	defer rows.Close()

	scores := map[string]CriteriaScore{}
	for rows.Next() {
		var callerID string
		var s CriteriaScore
		if err := rows.Scan(&callerID, &s.MetTrue, &s.Applicable); err != nil {
			return nil, fmt.Errorf("failed to scan criteria score: %w", err)
		}
		scores[callerID] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating criteria scores: %w", err)
	}
	return scores, nil
}

// CountFraudSuspectedByCaller returns, for every caller with calls in
// [from, to), how many of their calls Hermes flagged as fraud_suspected
// (operator hit an answering machine/voicemail and didn't hang up promptly
// — see the ANALYSIS_RUBRIC's fraud_suspected field in
// hermes/services/call_analytics_server.py) — one query across all callers,
// mirroring CountOutcomesByCaller, for surfacing suspected time-padding
// without an N+1 fetch. This is a separate boolean axis from `outcome`, not
// one of its values, per the regulation's separation of technical/fraud
// detection from script-compliance scoring.
// CountFraudSuspectedByCaller — per-caller fraud count for the dashboard
// badge (задача 9-adjacent, 2026-09-29). COALESCE prefers the new
// transcription-time precheck (fraud_precheck_suspected — see
// computeFraudPrecheck in transcribe/client.go) over the old
// analytics_json.fraud_suspected, but only for calls that have actually been
// through the new pipeline (fraud_precheck_suspected IS NOT NULL); every
// other call still falls back to the old signal. Switching the whole badge
// to the new field outright would have made counts silently drop for the
// ~630 September calls the new precheck hasn't reprocessed yet — not
// because they aren't fraud, just because nobody's asked the new pipeline
// about them. This bridges old→new one retry at a time instead of a hard
// cutover.
func (db *DB) CountFraudSuspectedByCaller(ctx context.Context, from, to time.Time) (map[string]int, error) {
	query := `
		SELECT caller_id, count(*)
		FROM calls
		WHERE caller_id IS NOT NULL AND started_at >= $1 AND started_at < $2
		  AND COALESCE(fraud_precheck_suspected, (analytics_json->>'fraud_suspected')::boolean) IS TRUE
		GROUP BY caller_id
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count fraud-suspected calls by caller: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var callerID string
		var count int
		if err := rows.Scan(&callerID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan fraud-suspected count: %w", err)
		}
		counts[callerID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fraud-suspected counts: %w", err)
	}
	return counts, nil
}

// ListCallsByStatusPeriod returns all calls (any caller) in [from, to)
// whose transcript_status is one of the given values — for a bulk "retry
// everything that didn't finish processing" job.
func (db *DB) ListCallsByStatusPeriod(ctx context.Context, statuses []string, from, to time.Time) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json,
		       COALESCE(engine, ''), transcribed_at, COALESCE(error_kind, ''), COALESCE(last_error, ''), created_at, updated_at
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
		var transcribedAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
			&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
			&c.TranscriptText, &analytics, &c.Engine, &transcribedAt, &c.ErrorKind, &c.LastError, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		if len(analytics) > 0 {
			c.AnalyticsJSON = json.RawMessage(analytics)
		}
		if transcribedAt.Valid {
			c.TranscribedAt = &transcribedAt.Time
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}
	return calls, nil
}

// ListCallsNeedingAnalysis returns calls in [from, to) whose transcript is
// ready (transcript_status='done') but haven't been LLM-classified yet
// (analytics_json IS NULL or missing both the legacy "category" key and the
// current "call_type" key) — the queue for the decoupled AnalyzeCalls job.
// Checking both keys matters: a call already analyzed under the old flat
// {category, outcome} shape (pre IQ-200-v1.2 rubric) is intentionally left
// as-is rather than backfilled, so it must still count as "analyzed" even
// though it lacks "call_type"; a call analyzed under the current shape must
// not be picked up again just because it lacks the legacy "category" key.
func (db *DB) ListCallsNeedingAnalysis(ctx context.Context, from, to time.Time) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json,
		       COALESCE(engine, ''), transcribed_at, COALESCE(error_kind, ''), COALESCE(last_error, ''), created_at, updated_at
		FROM calls
		WHERE transcript_status = 'done'
		  AND (analytics_json IS NULL OR NOT (analytics_json ? 'category' OR analytics_json ? 'call_type' OR analytics_json ? 'criteria'))
		  AND (error_kind IS NULL OR error_kind != 'hallucination_suspected')
		  -- 2026-09-29: fraud candidates (see transcribe.computeFraudPrecheck)
		  -- never reach the 4-criteria rubric at all — a call nobody real
		  -- answered has nothing for "умение слушать"/"работа с
		  -- возражениями" to measure, per management's own call. Exclusion
		  -- is on the boolean, not on fraud_review_status, so a call stays
		  -- excluded even before a human has confirmed/rejected it.
		  AND fraud_precheck_suspected IS NOT TRUE
		  AND started_at >= $1 AND started_at < $2
		ORDER BY started_at
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls needing analysis: %w", err)
	}
	defer rows.Close()

	var calls []models.Call
	for rows.Next() {
		var c models.Call
		var analytics []byte
		var transcribedAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
			&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
			&c.TranscriptText, &analytics, &c.Engine, &transcribedAt, &c.ErrorKind, &c.LastError, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		if len(analytics) > 0 {
			c.AnalyticsJSON = json.RawMessage(analytics)
		}
		if transcribedAt.Valid {
			c.TranscribedAt = &transcribedAt.Time
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}
	return calls, nil
}

// ListCallsForRetranscribe returns every call in [from, to), for the "GPU
// retranscribe" bulk job — regardless of transcript_status, since the point
// is to redo transcripts that already exist too. When onlyCPU is true,
// excludes calls already transcribed on GPU (the default in the UI's
// confirmation dialog — see zvonari-improvements.md, задача 4).
func (db *DB) ListCallsForRetranscribe(ctx context.Context, from, to time.Time, onlyCPU bool) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json,
		       COALESCE(engine, ''), transcribed_at, COALESCE(error_kind, ''), COALESCE(last_error, ''), created_at, updated_at
		FROM calls
		WHERE started_at >= $1 AND started_at < $2
	`
	if onlyCPU {
		query += ` AND engine IS DISTINCT FROM 'gpu'`
	}
	query += ` ORDER BY started_at`

	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query calls to retranscribe: %w", err)
	}
	defer rows.Close()

	var calls []models.Call
	for rows.Next() {
		var c models.Call
		var analytics []byte
		var transcribedAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
			&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
			&c.TranscriptText, &analytics, &c.Engine, &transcribedAt, &c.ErrorKind, &c.LastError, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		if len(analytics) > 0 {
			c.AnalyticsJSON = json.RawMessage(analytics)
		}
		if transcribedAt.Valid {
			c.TranscribedAt = &transcribedAt.Time
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}
	return calls, nil
}

// RetranscribePreviewCounts reports, for the "Перетранскрибировать на GPU"
// confirmation dialog, how many calls in [from, to) would be re-run under
// each of the two possible scopes (all calls vs only ones not yet done on
// GPU), plus the average call duration to size a rough time estimate from —
// see zvonari.Service.RetranscribePreview.
func (db *DB) RetranscribePreviewCounts(ctx context.Context, from, to time.Time) (total, alreadyGPU int, onlyCPUTotal int, avgDurationSec float64, err error) {
	query := `
		SELECT
			count(*),
			count(*) FILTER (WHERE engine = 'gpu'),
			count(*) FILTER (WHERE engine IS DISTINCT FROM 'gpu'),
			COALESCE(avg(duration_sec), 0)
		FROM calls
		WHERE started_at >= $1 AND started_at < $2
	`
	err = db.QueryRowContext(ctx, query, from, to).Scan(&total, &alreadyGPU, &onlyCPUTotal, &avgDurationSec)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to count calls for retranscribe preview: %w", err)
	}
	return total, alreadyGPU, onlyCPUTotal, avgDurationSec, nil
}

func (db *DB) ListCallsByCallerPeriod(ctx context.Context, callerID string, from, to time.Time) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json,
		       COALESCE(engine, ''), transcribed_at, COALESCE(error_kind, ''), COALESCE(last_error, ''), created_at, updated_at,
		       fraud_precheck_suspected, COALESCE(fraud_precheck_check, ''), COALESCE(fraud_precheck_quote, ''),
		       COALESCE(fraud_precheck_position, ''), COALESCE(fraud_review_status, ''), fraud_reviewed_at
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
		var transcribedAt, fraudReviewedAt sql.NullTime
		var fraudSuspected sql.NullBool
		if err := rows.Scan(
			&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
			&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
			&c.TranscriptText, &analytics, &c.Engine, &transcribedAt, &c.ErrorKind, &c.LastError, &c.CreatedAt, &c.UpdatedAt,
			&fraudSuspected, &c.FraudPrecheckCheck, &c.FraudPrecheckQuote, &c.FraudPrecheckPosition, &c.FraudReviewStatus, &fraudReviewedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		if len(analytics) > 0 {
			c.AnalyticsJSON = json.RawMessage(analytics)
		}
		if transcribedAt.Valid {
			c.TranscribedAt = &transcribedAt.Time
		}
		if fraudSuspected.Valid {
			c.FraudPrecheckSuspected = &fraudSuspected.Bool
		}
		if fraudReviewedAt.Valid {
			c.FraudReviewedAt = &fraudReviewedAt.Time
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calls: %w", err)
	}
	return calls, nil
}

// ListFraudCandidates returns every call awaiting human fraud review
// (fraud_review_status = 'pending') in [from, to), across all callers — the
// queue for задача 7's confirm/reject UI. Ordered oldest-first so a reviewer
// working through the queue clears the backlog in call order, not newest-
// first (which would leave old candidates to rot at the bottom indefinitely).
func (db *DB) ListFraudCandidates(ctx context.Context, from, to time.Time) ([]models.Call, error) {
	query := `
		SELECT id, pbx_uuid, caller_id, direction, counterparty_number, started_at, duration_sec, talk_time_sec,
		       hangup_cause, transcript_status, COALESCE(transcript_text, ''), analytics_json,
		       COALESCE(engine, ''), transcribed_at, COALESCE(error_kind, ''), COALESCE(last_error, ''), created_at, updated_at,
		       fraud_precheck_suspected, COALESCE(fraud_precheck_check, ''), COALESCE(fraud_precheck_quote, ''),
		       COALESCE(fraud_precheck_position, ''), COALESCE(fraud_review_status, ''), fraud_reviewed_at
		FROM calls
		WHERE fraud_review_status = 'pending' AND started_at >= $1 AND started_at < $2
		ORDER BY started_at
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query fraud candidates: %w", err)
	}
	defer rows.Close()

	var calls []models.Call
	for rows.Next() {
		var c models.Call
		var analytics []byte
		var transcribedAt, fraudReviewedAt sql.NullTime
		var fraudSuspected sql.NullBool
		if err := rows.Scan(
			&c.ID, &c.PBXUUID, &c.CallerID, &c.Direction, &c.CounterpartyNumber,
			&c.StartedAt, &c.DurationSec, &c.TalkTimeSec, &c.HangupCause, &c.TranscriptStatus,
			&c.TranscriptText, &analytics, &c.Engine, &transcribedAt, &c.ErrorKind, &c.LastError, &c.CreatedAt, &c.UpdatedAt,
			&fraudSuspected, &c.FraudPrecheckCheck, &c.FraudPrecheckQuote, &c.FraudPrecheckPosition, &c.FraudReviewStatus, &fraudReviewedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		if len(analytics) > 0 {
			c.AnalyticsJSON = json.RawMessage(analytics)
		}
		if transcribedAt.Valid {
			c.TranscribedAt = &transcribedAt.Time
		}
		if fraudSuspected.Valid {
			c.FraudPrecheckSuspected = &fraudSuspected.Bool
		}
		if fraudReviewedAt.Valid {
			c.FraudReviewedAt = &fraudReviewedAt.Time
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fraud candidates: %w", err)
	}
	return calls, nil
}

// FraudExportRow is one row for the xlsx export (задача: "кнопка выгрузки
// фрод-звонков в формате xlsx") — a flat, join-resolved shape (caller name
// instead of caller_id) since that's what belongs in a spreadsheet a human
// reads, not the FK a UI component looks up separately.
type FraudExportRow struct {
	ID           string
	StartedAt    time.Time
	CallerName   string
	TalkTimeSec  int
	DurationSec  int
	Quote        string
	Position     string
	ReviewStatus string
	Transcript   string
}

// ListFraudExportRows returns every call the precheck has ever flagged
// (fraud_precheck_suspected = true) in [from, to), regardless of
// fraud_review_status — unlike ListFraudCandidates (pending only), the
// export is meant as a full audit trail including what's already been
// confirmed/rejected, so a reviewer working from the spreadsheet can see the
// whole picture, not just what's left to do.
func (db *DB) ListFraudExportRows(ctx context.Context, from, to time.Time) ([]FraudExportRow, error) {
	query := `
		SELECT ca.id, ca.started_at, COALESCE(c.name, ''), ca.talk_time_sec, ca.duration_sec,
		       COALESCE(ca.fraud_precheck_quote, ''), COALESCE(ca.fraud_precheck_position, ''),
		       COALESCE(ca.fraud_review_status, ''), COALESCE(ca.transcript_text, '')
		FROM calls ca
		LEFT JOIN callers c ON c.id = ca.caller_id
		WHERE ca.fraud_precheck_suspected = TRUE AND ca.started_at >= $1 AND ca.started_at < $2
		ORDER BY c.name, ca.started_at
	`
	rows, err := db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query fraud export rows: %w", err)
	}
	defer rows.Close()

	var out []FraudExportRow
	for rows.Next() {
		var r FraudExportRow
		if err := rows.Scan(&r.ID, &r.StartedAt, &r.CallerName, &r.TalkTimeSec, &r.DurationSec,
			&r.Quote, &r.Position, &r.ReviewStatus, &r.Transcript); err != nil {
			return nil, fmt.Errorf("failed to scan fraud export row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fraud export rows: %w", err)
	}
	return out, nil
}

// SetFraudReviewStatus records a human's confirm/reject decision on a fraud
// candidate — the only thing that ever moves fraud_review_status out of
// 'pending' (see SetCallTranscript, which only ever sets 'pending', never
// 'confirmed'/'rejected'). status must be "confirmed" or "rejected"; the
// caller (handler) validates this before it reaches here.
func (db *DB) SetFraudReviewStatus(ctx context.Context, id, status string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE calls SET fraud_review_status = $2, fraud_reviewed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, status,
	)
	if err != nil {
		return fmt.Errorf("failed to update fraud review status: %w", err)
	}
	return nil
}
