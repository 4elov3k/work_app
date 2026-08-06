// Package zvonari orchestrates the "Звонари" feature: pulling call data from
// OnlinePBX, handing recordings to Hermes for local Whisper transcription,
// and Hermes' ChatGPT-based per-call and per-period analytics on top of the
// resulting text.
package zvonari

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"invoices-backend/internal/callreport"
	"invoices-backend/internal/database"
	"invoices-backend/internal/models"
	"invoices-backend/internal/pbx"
	"invoices-backend/internal/transcribe"
)

type Service struct {
	db         *database.DB
	pbx        *pbx.Client
	transcribe *transcribe.Client
	callreport *callreport.Client
}

func NewService(db *database.DB, pbxClient *pbx.Client, transcribeClient *transcribe.Client, callreportClient *callreport.Client) *Service {
	return &Service{db: db, pbx: pbxClient, transcribe: transcribeClient, callreport: callreportClient}
}

func (s *Service) Configured() bool {
	return s.pbx.Configured()
}

// cancelledHangupCause is OnlinePBX's hangup_cause for a call the originating
// side hung up before it connected. The user's "valid call" criteria
// (>=10s, not cancelled) excludes these even though the API's duration_from
// filter alone doesn't — a cancel can still take 10+ seconds of ringing.
const cancelledHangupCause = "ORIGINATOR_CANCEL"

// SyncCallers refreshes the callers table from OnlinePBX's live extension
// list, so a renamed or disabled agent is reflected without a manual step.
func (s *Service) SyncCallers(ctx context.Context) (int, error) {
	if !s.pbx.Configured() {
		return 0, fmt.Errorf("PBX не настроен (нет PBX_API_TOKEN/PBX_DOMAIN)")
	}
	users, err := s.pbx.ListUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("pbx.ListUsers: %w", err)
	}
	for _, u := range users {
		if u.Num == "" {
			continue
		}
		name := u.Name
		if name == "" || name == "-" {
			name = u.Num
		}
		if _, err := s.db.UpsertCaller(ctx, u.Num, name, u.Enabled); err != nil {
			return 0, fmt.Errorf("upserting caller %s: %w", u.Num, err)
		}
	}
	return len(users), nil
}

// callerExtension resolves which internal extension actually handled the
// call: for outbound calls that's caller_id_number (the agent's own
// number); for inbound/local it's the last "user"-type event, since
// destination_number is the raw dialled number, not who picked up.
func callerExtension(rec pbx.CallRecord) string {
	if rec.Accountcode == "outbound" {
		return rec.CallerIDNumber
	}
	var last string
	for _, ev := range rec.Events {
		if ev.Type == "user" {
			last = ev.Number
		}
	}
	return last
}

// SyncResult — итог одного запуска SyncCalls
type SyncResult struct {
	CallersSynced    int
	CallsFound       int
	CallsNew         int
	CallsSkipped     int
	TranscribeErrors int
}

// SyncCalls pulls CDR for [from, to) from OnlinePBX, inserts new valid calls,
// and (best-effort, per call) transcribes + analyzes each one via Hermes.
// A transcription/analytics failure never rolls back the CDR row already
// written — the call stays visible with transcript_status reflecting what
// went wrong, and a re-sync of the same period retries it, since pbx_uuid
// uniqueness only blocks re-inserting the CDR row, not re-processing a row
// stuck on "failed"/"no_recording".
func (s *Service) SyncCalls(ctx context.Context, from, to time.Time) (*SyncResult, error) {
	if !s.pbx.Configured() {
		return nil, fmt.Errorf("PBX не настроен (нет PBX_API_TOKEN/PBX_DOMAIN)")
	}

	callersSynced, err := s.SyncCallers(ctx)
	if err != nil {
		return nil, err
	}

	records, err := s.pbx.SearchHistory(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("pbx.SearchHistory: %w", err)
	}

	result := &SyncResult{CallersSynced: callersSynced, CallsFound: len(records)}

	for _, rec := range records {
		if rec.HangupCause == cancelledHangupCause {
			result.CallsSkipped++
			continue
		}

		ext := callerExtension(rec)
		var callerID *string
		if ext != "" {
			if caller, err := s.db.GetCallerByExtension(ctx, ext); err == nil && caller != nil {
				callerID = &caller.ID
			}
		}

		call, inserted, err := s.db.InsertCall(ctx, models.Call{
			PBXUUID:            rec.UUID,
			CallerID:           callerID,
			Direction:          rec.Accountcode,
			CounterpartyNumber: rec.DestinationNumber,
			StartedAt:          time.Unix(rec.StartStamp, 0).UTC(),
			DurationSec:        rec.Duration,
			TalkTimeSec:        rec.UserTalkTime,
			HangupCause:        rec.HangupCause,
		})
		if err != nil {
			return result, fmt.Errorf("inserting call %s: %w", rec.UUID, err)
		}
		if !inserted {
			result.CallsSkipped++
			continue
		}
		result.CallsNew++

		s.transcribeAndAnalyze(ctx, call, rec.UUID, result)
	}

	return result, nil
}

func (s *Service) transcribeAndAnalyze(ctx context.Context, call *models.Call, pbxUUID string, result *SyncResult) {
	if !s.transcribe.Configured() {
		return
	}

	audio, err := s.pbx.DownloadRecording(ctx, pbxUUID)
	if err != nil {
		log.Printf("zvonari: no recording for call %s: %v", pbxUUID, err)
		_ = s.db.SetCallTranscriptStatus(ctx, call.ID, "no_recording")
		return
	}

	_ = s.db.SetCallTranscriptStatus(ctx, call.ID, "transcribing")
	tr, err := s.transcribe.Transcribe(ctx, pbxUUID+".mp3", audio)
	if err != nil {
		log.Printf("zvonari: transcribe failed for call %s: %v", pbxUUID, err)
		_ = s.db.SetCallTranscriptStatus(ctx, call.ID, "failed")
		result.TranscribeErrors++
		return
	}
	if err := s.db.SetCallTranscript(ctx, call.ID, tr.Text); err != nil {
		log.Printf("zvonari: saving transcript failed for call %s: %v", pbxUUID, err)
		return
	}

	if !s.callreport.Configured() {
		return
	}
	analysis, err := s.callreport.AnalyzeCall(ctx, callreport.AnalyzeCallRequest{CallID: call.ID, Transcript: tr.Text})
	if err != nil {
		log.Printf("zvonari: call analysis failed for call %s: %v", pbxUUID, err)
		return
	}
	if err := s.db.SetCallAnalytics(ctx, call.ID, analysis.AnalyticsJSON); err != nil {
		log.Printf("zvonari: saving analytics failed for call %s: %v", pbxUUID, err)
	}
}

// RequestCallerReport aggregates a caller's transcribed calls for [from, to),
// asks Hermes to synthesize a period summary, and persists the result.
func (s *Service) RequestCallerReport(ctx context.Context, callerID, period string, from, to time.Time) (*models.CallerReport, error) {
	if !s.callreport.Configured() {
		return nil, fmt.Errorf("аналитика недоступна (не настроен CALL_ANALYTICS_URL)")
	}

	caller, err := s.db.GetCallerByID(ctx, callerID)
	if err != nil {
		return nil, err
	}

	calls, err := s.db.ListCallsByCallerPeriod(ctx, callerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing calls: %w", err)
	}

	req := callreport.GenerateReportRequest{
		CallerName:  caller.Name,
		Period:      period,
		PeriodStart: from.Format("2006-01-02"),
		PeriodEnd:   to.Format("2006-01-02"),
	}
	for _, c := range calls {
		if c.TranscriptText == "" {
			continue
		}
		req.Calls = append(req.Calls, callreport.CallForReport{
			CallID:        c.ID,
			StartedAt:     c.StartedAt.Format(time.RFC3339),
			Direction:     c.Direction,
			Transcript:    c.TranscriptText,
			AnalyticsJSON: c.AnalyticsJSON,
		})
	}
	if len(req.Calls) == 0 {
		return nil, fmt.Errorf("нет расшифрованных звонков за этот период")
	}

	genResult, err := s.callreport.GenerateReport(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("hermes generate-report: %w", err)
	}

	report, err := s.db.CreateCallerReport(ctx, database.CreateCallerReportParams{
		CallerID:    callerID,
		Period:      period,
		PeriodStart: from.Format("2006-01-02"),
		PeriodEnd:   to.Format("2006-01-02"),
		SummaryText: genResult.SummaryText,
		MetricsJSON: genResult.MetricsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("saving report: %w", err)
	}
	return report, nil
}

// CallDistribution buckets a caller's calls for a period by their Hermes
// outcome classification (analytics_json.outcome) — the UI's "brief
// distribution" view. No Hermes call needed: it aggregates analysis that
// already happened when each call was synced.
func (s *Service) CallDistribution(ctx context.Context, callerID string, from, to time.Time) (map[string]int, error) {
	calls, err := s.db.ListCallsByCallerPeriod(ctx, callerID, from, to)
	if err != nil {
		return nil, err
	}
	dist := map[string]int{}
	for _, c := range calls {
		dist[extractOutcome(c.AnalyticsJSON)]++
	}
	return dist, nil
}

func extractOutcome(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "не проанализировано"
	}
	var parsed struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.Outcome == "" {
		return "не проанализировано"
	}
	return parsed.Outcome
}
