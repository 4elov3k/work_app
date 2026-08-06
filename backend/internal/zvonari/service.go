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
	"sync"
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

	syncMu     sync.Mutex
	syncStatus SyncStatus
}

func NewService(db *database.DB, pbxClient *pbx.Client, transcribeClient *transcribe.Client, callreportClient *callreport.Client) *Service {
	return &Service{db: db, pbx: pbxClient, transcribe: transcribeClient, callreport: callreportClient}
}

func (s *Service) Configured() bool {
	return s.pbx.Configured()
}

// SyncStatus reports the state of the (at most one) in-flight or last
// completed sync run, for the frontend to poll instead of holding an HTTP
// request open for the whole batch. TotalToProcess/Processed cover only the
// transcribe+analyze phase (the slow part) — CDR fetch/insert is fast
// enough that a coarse "still fetching CDR" vs "processing N/M" distinction
// (Processed==0 vs >0) is all the UI needs.
type SyncStatus struct {
	Running        bool        `json:"running"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	FinishedAt     *time.Time  `json:"finished_at,omitempty"`
	TotalToProcess int         `json:"total_to_process,omitempty"`
	Processed      int         `json:"processed,omitempty"`
	Result         *SyncResult `json:"result,omitempty"`
	Error          string      `json:"error,omitempty"`
}

// StartSync launches a sync in the background and returns immediately.
// Returns false if one is already running (never runs two concurrently —
// OnlinePBX/Hermes get hammered otherwise, and duplicate concurrent inserts
// would race on the same pbx_uuid). The background run uses its own
// context, detached from the triggering HTTP request, so a client
// disconnect (browser closed, proxy idle-timeout) can't abort a batch
// that's minutes into transcribing/analyzing calls — a full sync over many
// calls easily exceeds typical reverse-proxy idle timeouts.
func (s *Service) StartSync(from, to time.Time) bool {
	s.syncMu.Lock()
	if s.syncStatus.Running {
		s.syncMu.Unlock()
		return false
	}
	startedAt := time.Now()
	s.syncStatus = SyncStatus{Running: true, StartedAt: &startedAt}
	s.syncMu.Unlock()

	go func() {
		result, err := s.SyncCalls(context.Background(), from, to)
		finishedAt := time.Now()

		s.syncMu.Lock()
		defer s.syncMu.Unlock()
		s.syncStatus.Running = false
		s.syncStatus.FinishedAt = &finishedAt
		s.syncStatus.Result = result
		if err != nil {
			log.Printf("zvonari: background sync failed: %v", err)
			s.syncStatus.Error = err.Error()
		} else {
			s.syncStatus.Error = ""
		}
	}()

	return true
}

// GetSyncStatus returns a snapshot of the current/last sync run.
func (s *Service) GetSyncStatus() SyncStatus {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	return s.syncStatus
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

// maxConcurrentProcessing bounds how many calls are transcribed+analyzed at
// once. Each step is a slow network round-trip (Whisper is CPU-bound on the
// Hermes side, ~10-20s/call; the analytics LLM call is another ~10-20s) —
// running them one at a time left the machine's other CPU cores idle for no
// reason, so a sync over a busy day took many minutes longer than it needed
// to. 4 keeps call-transcribe's own per-request thread cap (2 CPU threads,
// see TRANSCRIBE_CPU_THREADS) from oversubscribing an 8-core host.
const maxConcurrentProcessing = 4

// SyncCalls pulls CDR for [from, to) from OnlinePBX, inserts new valid calls,
// and (best-effort, per call) transcribes + analyzes each one via Hermes —
// several calls concurrently (see maxConcurrentProcessing) rather than one
// at a time. A transcription/analytics failure never rolls back the CDR row
// already written — the call stays visible with transcript_status
// reflecting what went wrong, and a re-sync of the same period retries it,
// since pbx_uuid uniqueness only blocks re-inserting the CDR row, not
// re-processing a row stuck on "failed"/"no_recording".
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

	type pending struct {
		call *models.Call
		uuid string
	}
	var toProcess []pending

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
		toProcess = append(toProcess, pending{call: call, uuid: rec.UUID})
	}

	s.syncMu.Lock()
	s.syncStatus.TotalToProcess = len(toProcess)
	s.syncStatus.Processed = 0
	s.syncMu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentProcessing)
	for _, p := range toProcess {
		wg.Add(1)
		sem <- struct{}{}
		go func(call *models.Call, uuid string) {
			defer wg.Done()
			defer func() { <-sem }()
			s.transcribeAndAnalyze(ctx, call, uuid, &mu, result)
			s.syncMu.Lock()
			s.syncStatus.Processed++
			s.syncMu.Unlock()
		}(p.call, p.uuid)
	}
	wg.Wait()

	return result, nil
}

func (s *Service) transcribeAndAnalyze(ctx context.Context, call *models.Call, pbxUUID string, mu *sync.Mutex, result *SyncResult) {
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
		mu.Lock()
		result.TranscribeErrors++
		mu.Unlock()
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

// RetranscribeCall (re)runs transcribe+analyze for one existing call, on
// demand from the UI — the only way that was previously possible was via a
// full sync, which only ever processes calls at insert time: a call stuck
// on "transcribing"/"failed" (e.g. because the backend restarted mid-batch)
// had no way to be retried short of that. Runs synchronously — a single
// call is seconds, not minutes, so it doesn't need the background-job
// treatment SyncCalls does.
func (s *Service) RetranscribeCall(ctx context.Context, callID string) (*models.Call, error) {
	call, err := s.db.GetCallByID(ctx, callID)
	if err != nil {
		return nil, err
	}

	var result SyncResult
	var mu sync.Mutex
	s.transcribeAndAnalyze(ctx, call, call.PBXUUID, &mu, &result)

	return s.db.GetCallByID(ctx, callID)
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

// CallCounts returns how many synced calls each caller has in [from, to) —
// one query for all callers, for the caller-list UI to show a count badge
// per card without a request per caller.
func (s *Service) CallCounts(ctx context.Context, from, to time.Time) (map[string]int, error) {
	return s.db.CountCallsByCallerPeriod(ctx, from, to)
}

// ListCalls returns a caller's individual calls for a period (with
// transcript + per-call analytics), for the UI's call-detail breakdown.
func (s *Service) ListCalls(ctx context.Context, callerID string, from, to time.Time) ([]models.Call, error) {
	return s.db.ListCallsByCallerPeriod(ctx, callerID, from, to)
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
