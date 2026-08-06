// Package zvonari orchestrates the "Звонари" feature: pulling call data from
// OnlinePBX, handing recordings to Hermes for local Whisper transcription,
// and Hermes' ChatGPT-based per-call and per-period analytics on top of the
// resulting text.
package zvonari

import (
	"context"
	"encoding/json"
	"errors"
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

// startBackgroundJob runs job in the background and returns immediately.
// Returns false if one is already running — a sync and a retry-failed run
// share this same slot so they never run concurrently against each other
// either (both hammer OnlinePBX/Hermes and both write to the same rows).
// The background run uses its own context, detached from the triggering
// HTTP request, so a client disconnect (browser closed, proxy idle-timeout)
// can't abort a batch that's minutes into transcribing/analyzing calls.
func (s *Service) startBackgroundJob(job func(ctx context.Context) (*SyncResult, error)) bool {
	s.syncMu.Lock()
	if s.syncStatus.Running {
		s.syncMu.Unlock()
		return false
	}
	startedAt := time.Now()
	s.syncStatus = SyncStatus{Running: true, StartedAt: &startedAt}
	s.syncMu.Unlock()

	go func() {
		result, err := job(context.Background())
		finishedAt := time.Now()

		s.syncMu.Lock()
		defer s.syncMu.Unlock()
		s.syncStatus.Running = false
		s.syncStatus.FinishedAt = &finishedAt
		s.syncStatus.Result = result
		if err != nil {
			log.Printf("zvonari: background job failed: %v", err)
			s.syncStatus.Error = err.Error()
		} else {
			s.syncStatus.Error = ""
		}
	}()

	return true
}

// StartSync launches a CDR sync in the background — see startBackgroundJob.
func (s *Service) StartSync(from, to time.Time) bool {
	return s.startBackgroundJob(func(ctx context.Context) (*SyncResult, error) {
		return s.SyncCalls(ctx, from, to)
	})
}

// StartRetryFailed re-attempts transcribe+analyze, in the background, for
// every existing call in [from, to) stuck on "failed"/"no_recording"/
// "pending"/"transcribing" — the bulk counterpart to RetranscribeCall, for
// clearing a backlog (e.g. after a bug fix, or a batch of backend restarts
// mid-sync) without clicking through each call one at a time.
func (s *Service) StartRetryFailed(from, to time.Time) bool {
	return s.startBackgroundJob(func(ctx context.Context) (*SyncResult, error) {
		return s.RetryFailedCalls(ctx, from, to)
	})
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

// SyncResult — итог одного запуска фоновой задачи (синк+транскрибация или
// анализ) — общий тип для всех задач, использующих один "слот"
// (startBackgroundJob), чтобы синк/повтор/анализ никогда не бежали
// одновременно и делили один и тот же прогресс-статус.
type SyncResult struct {
	CallersSynced    int
	CallsFound       int
	CallsNew         int
	CallsSkipped     int
	TranscribeErrors int
	AnalyzeErrors    int
}

// maxConcurrentProcessing bounds how many calls are transcribed at once.
// Measured directly on this host (isolated, in-process, not confounded by
// analytics or network overhead): 1 stream at cpu_threads=2 transcribes at
// 2.07x realtime; 2 concurrent streams manage only 2.21x *aggregate* — i.e.
// concurrency buys almost nothing here, this Docker VM's CPU scheduling
// just doesn't scale past a single well-tuned stream. Pushing to 4
// concurrent (an earlier attempt) made it actively worse — full
// oversubscription collapsed aggregate throughput to ~1x. Given that
// ceiling, sequential processing (1) is simpler and avoids the
// timeout/contention failures concurrency was causing, for no real speed
// cost — see decision to decouple transcription from analytics below.
const maxConcurrentProcessing = 1

// pending is a call queued for transcribe+analyze, shared by SyncCalls
// (freshly-inserted CDR rows) and RetryFailedCalls (existing rows stuck on
// a non-"done" status).
type pending struct {
	call *models.Call
	uuid string
}

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

	s.processConcurrently(ctx, toProcess, result)

	return result, nil
}

// RetryFailedCalls re-runs transcribe+analyze for every existing call in
// [from, to) whose transcript_status isn't "done" — the bulk counterpart to
// a single RetranscribeCall, for clearing a backlog in one go.
func (s *Service) RetryFailedCalls(ctx context.Context, from, to time.Time) (*SyncResult, error) {
	calls, err := s.db.ListCallsByStatusPeriod(ctx, []string{"failed", "no_recording", "pending", "transcribing"}, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing calls to retry: %w", err)
	}

	result := &SyncResult{CallsFound: len(calls)}
	var toProcess []pending
	for i := range calls {
		toProcess = append(toProcess, pending{call: &calls[i], uuid: calls[i].PBXUUID})
	}

	s.processConcurrently(ctx, toProcess, result)

	return result, nil
}

// processConcurrently runs transcribeAndAnalyze for each pending call, up
// to maxConcurrentProcessing at once, and keeps the live sync-status
// progress counters (TotalToProcess/Processed) up to date as it goes.
func (s *Service) processConcurrently(ctx context.Context, toProcess []pending, result *SyncResult) {
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
			s.transcribeOnly(ctx, call, uuid, &mu, result)
			s.syncMu.Lock()
			s.syncStatus.Processed++
			s.syncMu.Unlock()
		}(p.call, p.uuid)
	}
	wg.Wait()
}

// transcribeOnly downloads and transcribes one call via local Whisper —
// deliberately does NOT call the LLM analytics step. Transcription and
// analysis are decoupled on purpose: transcription is the CPU-bound,
// time-sensitive batch (run once a day, ideally via cron, sequentially —
// see maxConcurrentProcessing), while classification can lag behind by
// hours without anyone noticing, so it runs as its own separate job
// (AnalyzeCalls) instead of blocking or being blocked by transcription.
func (s *Service) transcribeOnly(ctx context.Context, call *models.Call, pbxUUID string, mu *sync.Mutex, result *SyncResult) {
	if !s.transcribe.Configured() {
		return
	}

	// One retry before giving up: an occasional OnlinePBX API hiccup (rate
	// limit, transient network error) previously got permanently misfiled
	// as "no recording" right alongside calls that genuinely never had
	// one — this only downgrades to ErrNoRecording (not retried, since
	// that means OnlinePBX explicitly confirmed there's nothing to fetch)
	// after a real second failure.
	audio, err := s.pbx.DownloadRecording(ctx, pbxUUID)
	if err != nil && !errors.Is(err, pbx.ErrNoRecording) {
		audio, err = s.pbx.DownloadRecording(ctx, pbxUUID)
	}
	if err != nil {
		if errors.Is(err, pbx.ErrNoRecording) {
			log.Printf("zvonari: no recording for call %s", pbxUUID)
			_ = s.db.SetCallTranscriptStatus(ctx, call.ID, "no_recording")
		} else {
			log.Printf("zvonari: failed to download recording for call %s: %v", pbxUUID, err)
			_ = s.db.SetCallTranscriptStatus(ctx, call.ID, "failed")
			mu.Lock()
			result.TranscribeErrors++
			mu.Unlock()
		}
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
	}
}

// AnalyzeCalls finds every call in [from, to) whose transcript is ready but
// hasn't been classified yet, and runs the LLM outcome classification for
// each — decoupled from transcription (see transcribeOnly), so it can run
// on its own schedule (e.g. a few hours after the morning transcription
// batch) without either job blocking the other.
func (s *Service) AnalyzeCalls(ctx context.Context, from, to time.Time) (*SyncResult, error) {
	if !s.callreport.Configured() {
		return nil, fmt.Errorf("аналитика недоступна (не настроен CALL_ANALYTICS_URL)")
	}

	calls, err := s.db.ListCallsNeedingAnalysis(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing calls needing analysis: %w", err)
	}

	result := &SyncResult{CallsFound: len(calls)}
	s.syncMu.Lock()
	s.syncStatus.TotalToProcess = len(calls)
	s.syncStatus.Processed = 0
	s.syncMu.Unlock()

	for i := range calls {
		call := &calls[i]
		analysis, err := s.callreport.AnalyzeCall(ctx, callreport.AnalyzeCallRequest{CallID: call.ID, Transcript: call.TranscriptText})
		if err != nil {
			log.Printf("zvonari: call analysis failed for call %s: %v", call.PBXUUID, err)
			result.AnalyzeErrors++
		} else if err := s.db.SetCallAnalytics(ctx, call.ID, analysis.AnalyticsJSON); err != nil {
			log.Printf("zvonari: saving analytics failed for call %s: %v", call.PBXUUID, err)
			result.AnalyzeErrors++
		}
		s.syncMu.Lock()
		s.syncStatus.Processed++
		s.syncMu.Unlock()
	}

	return result, nil
}

// StartAnalyzeCalls launches AnalyzeCalls in the background — see startBackgroundJob.
func (s *Service) StartAnalyzeCalls(from, to time.Time) bool {
	return s.startBackgroundJob(func(ctx context.Context) (*SyncResult, error) {
		return s.AnalyzeCalls(ctx, from, to)
	})
}

// RetranscribeCall (re)runs transcription for one existing call, on demand
// from the UI — the only way that was previously possible was via a full
// sync, which only ever processes calls at insert time: a call stuck on
// "transcribing"/"failed" (e.g. because the backend restarted mid-batch)
// had no way to be retried short of that. Runs synchronously — a single
// call is seconds, not minutes, so it doesn't need the background-job
// treatment SyncCalls does. Does NOT re-run analysis — call AnalyzeCalls
// separately if the transcript changed and needs re-classifying.
func (s *Service) RetranscribeCall(ctx context.Context, callID string) (*models.Call, error) {
	call, err := s.db.GetCallByID(ctx, callID)
	if err != nil {
		return nil, err
	}

	var result SyncResult
	var mu sync.Mutex
	s.transcribeOnly(ctx, call, call.PBXUUID, &mu, &result)

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

// CallStatusCounts returns, per caller, a breakdown of calls by
// transcript_status in [from, to) — the "полная статистика" view showing
// how many actually finished processing vs are stuck/failed, not just a
// raw call count.
func (s *Service) CallStatusCounts(ctx context.Context, from, to time.Time) (map[string]map[string]int, error) {
	return s.db.CountCallsByCallerAndStatus(ctx, from, to)
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
