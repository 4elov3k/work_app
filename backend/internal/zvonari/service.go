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

	// Pause state for the currently running background job — separate lock
	// from syncMu so GetSyncStatus never has to take both at once. Pausing
	// doesn't cancel anything in flight (maxConcurrentProcessing is 1
	// anyway); it just stops the loop from starting the *next* call/batch
	// until resumed, via a blocking receive on resumeCh.
	pauseMu  sync.Mutex
	paused   bool
	resumeCh chan struct{}

	// cancelFunc cancels the context passed into the currently running
	// background job, if any — set by startBackgroundJob, cleared once the
	// job's goroutine returns. Guarded by syncMu alongside syncStatus, since
	// "is a job running" and "how do I stop it" are the same piece of state.
	cancelFunc context.CancelFunc

	// stages holds the last known state of each pipeline phase
	// (sync/transcribe/analyze), guarded by syncMu like syncStatus —
	// separate from it because a new job overwrites syncStatus wholesale on
	// start (see startBackgroundJob) but stages must persist across
	// different job kinds: starting an analyze run shouldn't blank out what
	// the last sync/transcribe run left behind (задача 1,
	// zvonari-improvements.md — see StageStatus).
	stages map[string]*StageStatus

	healthMu       sync.Mutex
	healthCache    *HealthStatus
	healthCachedAt time.Time
}

func NewService(db *database.DB, pbxClient *pbx.Client, transcribeClient *transcribe.Client, callreportClient *callreport.Client) *Service {
	return &Service{
		db: db, pbx: pbxClient, transcribe: transcribeClient, callreport: callreportClient,
		stages: map[string]*StageStatus{
			"sync":       {State: StageIdle},
			"transcribe": {State: StageIdle},
			"analyze":    {State: StageIdle},
		},
	}
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
	Paused         bool        `json:"paused"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	FinishedAt     *time.Time  `json:"finished_at,omitempty"`
	TotalToProcess int         `json:"total_to_process,omitempty"`
	Processed      int         `json:"processed,omitempty"`
	Result         *SyncResult `json:"result,omitempty"`
	Error          string      `json:"error,omitempty"`
	// Stages breaks the single Processed/TotalToProcess pair above into the
	// three actual pipeline phases (задача 1, zvonari-improvements.md) — the
	// frontend renders one bar per stage instead of one bar whose meaning
	// silently depends on which button was last clicked. Kept alongside the
	// flat fields above rather than replacing them (additive change).
	Stages map[string]StageStatus `json:"stages,omitempty"`
}

// StageState is one segment of the pipeline: sync (CDR fetch), transcribe
// (Whisper), analyze (Hermes classification) — see задача 1.
type StageState string

const (
	StageIdle    StageState = "idle"
	StageRunning StageState = "running"
	StageDone    StageState = "done"
	StageFailed  StageState = "failed"
)

// StageStatus is the state of one pipeline phase, independent of which job
// (sync/retry-failed/retranscribe-gpu/analyze) is currently running — e.g.
// "transcribe" reflects the last transcription work regardless of whether
// it ran as part of a full sync or a standalone retry.
type StageStatus struct {
	State      StageState `json:"state"`
	Done       int        `json:"done,omitempty"`
	Total      int        `json:"total,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// markStageRunning resets one stage to a fresh in-progress run — called at
// the point each phase actually starts doing work (not necessarily at job
// start: SyncCalls starts "sync" immediately but "transcribe" only once CDR
// fetch finishes and it knows how many calls need processing).
func (s *Service) markStageRunning(name string) {
	now := time.Now()
	s.syncMu.Lock()
	s.stages[name] = &StageStatus{State: StageRunning, StartedAt: &now}
	s.syncMu.Unlock()
}

// markStageDone marks a stage finished successfully with a final done/total.
func (s *Service) markStageDone(name string, done, total int) {
	now := time.Now()
	s.syncMu.Lock()
	st := s.stages[name]
	if st == nil {
		st = &StageStatus{}
		s.stages[name] = st
	}
	st.State = StageDone
	st.Done = done
	st.Total = total
	st.FinishedAt = &now
	s.syncMu.Unlock()
}

// markStageFailed marks a stage finished unsuccessfully, keeping whatever
// Done/Total it had accumulated so far (a partial run still shows progress).
func (s *Service) markStageFailed(name string) {
	now := time.Now()
	s.syncMu.Lock()
	st := s.stages[name]
	if st == nil {
		st = &StageStatus{}
		s.stages[name] = st
	}
	st.State = StageFailed
	st.FinishedAt = &now
	s.syncMu.Unlock()
}

// snapshotStages copies the stages map for a GetSyncStatus response — must
// be called with syncMu held, since StageStatus values (not pointers) are
// what cross the API boundary so a caller can never observe a torn read of
// a stage still being mutated concurrently.
func (s *Service) snapshotStagesLocked() map[string]StageStatus {
	out := make(map[string]StageStatus, len(s.stages))
	for k, v := range s.stages {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// startBackgroundJob runs job in the background and returns immediately.
// Returns false if one is already running — a sync and a retry-failed run
// share this same slot so they never run concurrently against each other
// either (both hammer OnlinePBX/Hermes and both write to the same rows).
// The background run uses a context detached from the triggering HTTP
// request (so a client disconnect can't abort a batch minutes into
// transcribing/analyzing calls) but derived from context.Background() via
// WithCancel, so an explicit Stop() call can still abort it — unlike Pause,
// which only stops the loop from starting the *next* item, Stop cancels
// whatever's in flight right now too.
func (s *Service) startBackgroundJob(job func(ctx context.Context) (*SyncResult, error)) bool {
	s.syncMu.Lock()
	if s.syncStatus.Running {
		s.syncMu.Unlock()
		return false
	}
	startedAt := time.Now()
	s.syncStatus = SyncStatus{Running: true, StartedAt: &startedAt}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.syncMu.Unlock()

	s.pauseMu.Lock()
	s.paused = false
	s.pauseMu.Unlock()

	go func() {
		result, err := job(ctx)
		finishedAt := time.Now()

		s.syncMu.Lock()
		defer s.syncMu.Unlock()
		s.syncStatus.Running = false
		s.syncStatus.FinishedAt = &finishedAt
		s.syncStatus.Result = result
		s.cancelFunc = nil
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("zvonari: background job stopped by user")
				s.syncStatus.Error = "остановлено пользователем"
			} else {
				log.Printf("zvonari: background job failed: %v", err)
				s.syncStatus.Error = err.Error()
			}
		} else {
			s.syncStatus.Error = ""
		}
	}()

	return true
}

// Stop cancels the currently running background job, if any — in-flight
// PBX/transcribe/analytics HTTP calls abort (their context is this same
// job context, see SyncCalls/RetryFailedCalls/RetranscribeAll/AnalyzeCalls),
// and the processing loops (processConcurrently, AnalyzeCalls' batch loop)
// stop starting new work once they notice ctx is done. Whatever a call/batch
// already finished before Stop was called stays saved — same durability
// guarantee Pause already relies on. Returns false if nothing was running.
func (s *Service) Stop() bool {
	s.syncMu.Lock()
	cancel := s.cancelFunc
	s.syncMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	// A paused job's loop is blocked on waitIfPaused's select, which also
	// listens on ctx.Done() — cancelling is enough to unblock it too, no
	// need to separately clear the pause flag.
	return true
}

// StartSync launches a CDR sync in the background — see startBackgroundJob.
func (s *Service) StartSync(from, to time.Time) bool {
	return s.startBackgroundJob(func(ctx context.Context) (*SyncResult, error) {
		return s.SyncCalls(ctx, from, to)
	})
}

// StartRetryFailed re-attempts transcribe+analyze, in the background, for
// every existing call in [from, to) stuck on a recoverable status — the bulk
// counterpart to RetranscribeCall, for clearing a backlog (e.g. after a bug
// fix, or a batch of backend restarts mid-sync) without clicking through
// each call one at a time. includeTerminal additionally retries
// "no_recording" calls — off by default, since a missing recording almost
// never becomes available by retrying (OnlinePBX already confirmed there's
// nothing to fetch) and previously made "Повторить неудачные" silently
// re-hammer calls that could never succeed (задача 6, zvonari-improvements.md).
func (s *Service) StartRetryFailed(from, to time.Time, includeTerminal bool) bool {
	return s.startBackgroundJob(func(ctx context.Context) (*SyncResult, error) {
		return s.RetryFailedCalls(ctx, from, to, includeTerminal)
	})
}

// GetSyncStatus returns a snapshot of the current/last sync run.
func (s *Service) GetSyncStatus() SyncStatus {
	s.syncMu.Lock()
	status := s.syncStatus
	status.Stages = s.snapshotStagesLocked()
	s.syncMu.Unlock()
	status.Paused = s.IsPaused()
	return status
}

// Pause stops the running background job (sync/retry/analyze) from starting
// its next call or batch, without cancelling anything already in flight or
// losing progress — each call/batch is already durably written to the DB as
// it completes, so pausing between iterations is always safe to resume from.
func (s *Service) Pause() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if !s.paused {
		s.paused = true
		s.resumeCh = make(chan struct{})
	}
}

// Resume releases a paused job to continue from where it left off.
func (s *Service) Resume() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if s.paused {
		s.paused = false
		close(s.resumeCh)
	}
}

func (s *Service) IsPaused() bool {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	return s.paused
}

// waitIfPaused blocks the calling goroutine while the job is paused, called
// between iterations of a processing loop (per call, per analyze batch).
func (s *Service) waitIfPaused(ctx context.Context) {
	for {
		s.pauseMu.Lock()
		if !s.paused {
			s.pauseMu.Unlock()
			return
		}
		ch := s.resumeCh
		s.pauseMu.Unlock()

		select {
		case <-ch:
		case <-ctx.Done():
			return
		}
	}
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
	CallersSynced    int `json:"callers_synced"`
	CallsFound       int `json:"calls_found"`
	CallsNew         int `json:"calls_new"`
	CallsSkipped     int `json:"calls_skipped"`
	TranscribeErrors int `json:"transcribe_errors"`
	AnalyzeErrors    int `json:"analyze_errors"`
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
	s.markStageRunning("sync")

	callersSynced, err := s.SyncCallers(ctx)
	if err != nil {
		s.markStageFailed("sync")
		return nil, err
	}

	records, err := s.pbx.SearchHistory(ctx, from, to)
	if err != nil {
		s.markStageFailed("sync")
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
			s.markStageFailed("sync")
			return result, fmt.Errorf("inserting call %s: %w", rec.UUID, err)
		}
		if !inserted {
			result.CallsSkipped++
			continue
		}
		result.CallsNew++
		toProcess = append(toProcess, pending{call: call, uuid: rec.UUID})
	}
	s.markStageDone("sync", len(records), len(records))

	s.processConcurrently(ctx, toProcess, result)

	return result, nil
}

// recoverableStatuses are transcript_status values where retrying can
// actually help — unlike "no_recording", where OnlinePBX already confirmed
// there's nothing to fetch, so a retry can only ever repeat the same result.
var recoverableStatuses = []string{"failed", "pending", "transcribing"}

// RetryFailedCalls re-runs transcribe+analyze for every existing call in
// [from, to) stuck on a recoverable status — the bulk counterpart to a
// single RetranscribeCall, for clearing a backlog in one go. includeTerminal
// additionally sweeps up "no_recording" calls, as a separate explicit choice
// rather than the default (задача 6, zvonari-improvements.md).
func (s *Service) RetryFailedCalls(ctx context.Context, from, to time.Time, includeTerminal bool) (*SyncResult, error) {
	statuses := recoverableStatuses
	if includeTerminal {
		statuses = allTranscriptStatuses
	}
	calls, err := s.db.ListCallsByStatusPeriod(ctx, statuses, from, to)
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

// allTranscriptStatuses covers every value transcript_status can hold —
// used by RetryFailedCalls's includeTerminal variant to also sweep up
// "no_recording" calls on top of the recoverable ones.
var allTranscriptStatuses = []string{"done", "failed", "no_recording", "pending", "transcribing"}

// RetranscribeAll force-retranscribes calls in [from, to] — including ones
// already transcript_status='done' — via transcribeOnly, which always
// prefers the GPU box (see transcribe.Client.Transcribe) when
// TRANSCRIBE_SERVICE_GPU_URL is configured and falls back to the local CPU
// service otherwise. Exists for backfilling better transcripts once a GPU
// box comes online after calls were already transcribed on CPU. onlyCPU
// restricts the run to calls not already transcribed on GPU — the default
// in the UI's confirmation dialog, since re-running calls already done on
// GPU is rarely useful and this is the most expensive operation in the
// system (see RetranscribePreview).
func (s *Service) RetranscribeAll(ctx context.Context, from, to time.Time, onlyCPU bool) (*SyncResult, error) {
	calls, err := s.db.ListCallsForRetranscribe(ctx, from, to, onlyCPU)
	if err != nil {
		return nil, fmt.Errorf("listing calls to retranscribe: %w", err)
	}

	result := &SyncResult{CallsFound: len(calls)}
	var toProcess []pending
	for i := range calls {
		toProcess = append(toProcess, pending{call: &calls[i], uuid: calls[i].PBXUUID})
	}

	s.processConcurrently(ctx, toProcess, result)

	return result, nil
}

// StartRetranscribeAll launches RetranscribeAll in the background — see startBackgroundJob.
func (s *Service) StartRetranscribeAll(from, to time.Time, onlyCPU bool) bool {
	return s.startBackgroundJob(func(ctx context.Context) (*SyncResult, error) {
		return s.RetranscribeAll(ctx, from, to, onlyCPU)
	})
}

// RetranscribePreview is what the frontend calls before actually starting a
// GPU retranscribe run, to show "будет пересчитано N звонков (из них M уже
// на GPU), ~T" instead of firing the most expensive operation in the system
// blind (see zvonari-improvements.md, задача 4). The time estimate is a
// rough one: total audio duration of the affected calls divided by the
// measured single-stream CPU throughput documented on maxConcurrentProcessing
// (~2x realtime) — GPU is faster but by an amount that varies by hardware
// and isn't measured anywhere in this codebase, so this deliberately quotes
// the more conservative (CPU) bound rather than a number that could
// undersell how long a CPU fallback run would actually take.
type RetranscribePreview struct {
	Total            int     `json:"total"`
	AlreadyGPU       int     `json:"already_gpu"`
	OnlyCPUTotal     int     `json:"only_cpu_total"`
	EstimatedMinutes float64 `json:"estimated_minutes"`
}

// assumedCPURealtimeFactor mirrors the 2.07x single-stream measurement
// documented on maxConcurrentProcessing above.
const assumedCPURealtimeFactor = 2.0

func (s *Service) RetranscribePreview(ctx context.Context, from, to time.Time, onlyCPU bool) (*RetranscribePreview, error) {
	total, alreadyGPU, onlyCPUTotal, avgDurationSec, err := s.db.RetranscribePreviewCounts(ctx, from, to)
	if err != nil {
		return nil, err
	}
	count := total
	if onlyCPU {
		count = onlyCPUTotal
	}
	estimatedMinutes := float64(count) * avgDurationSec / assumedCPURealtimeFactor / 60
	return &RetranscribePreview{
		Total:            total,
		AlreadyGPU:       alreadyGPU,
		OnlyCPUTotal:     onlyCPUTotal,
		EstimatedMinutes: estimatedMinutes,
	}, nil
}

// processConcurrently runs transcribeAndAnalyze for each pending call, up
// to maxConcurrentProcessing at once, and keeps the live sync-status
// progress counters (TotalToProcess/Processed) up to date as it goes.
func (s *Service) processConcurrently(ctx context.Context, toProcess []pending, result *SyncResult) {
	now := time.Now()
	s.syncMu.Lock()
	s.syncStatus.TotalToProcess = len(toProcess)
	s.syncStatus.Processed = 0
	s.stages["transcribe"] = &StageStatus{State: StageRunning, Total: len(toProcess), StartedAt: &now}
	s.syncMu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentProcessing)
	for _, p := range toProcess {
		s.waitIfPaused(ctx)
		// Stop() cancels ctx — without this check the loop would keep
		// spawning goroutines that immediately fail (ctx already done),
		// misrecording each remaining call as a real download/transcribe
		// error instead of just "never attempted".
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(call *models.Call, uuid string) {
			defer wg.Done()
			defer func() { <-sem }()
			s.transcribeOnly(ctx, call, uuid, &mu, result)
			s.syncMu.Lock()
			s.syncStatus.Processed++
			s.stages["transcribe"].Done++
			s.syncMu.Unlock()
		}(p.call, p.uuid)
	}
	wg.Wait()
	if ctx.Err() != nil {
		s.markStageFailed("transcribe")
	} else {
		s.markStageDone("transcribe", len(toProcess), len(toProcess))
	}
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
			_ = s.db.SetCallTranscriptError(ctx, call.ID, "no_recording", "no_recording", "")
		} else {
			log.Printf("zvonari: failed to download recording for call %s: %v", pbxUUID, err)
			_ = s.db.SetCallTranscriptError(ctx, call.ID, "failed", "download_failed", err.Error())
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
		_ = s.db.SetCallTranscriptError(ctx, call.ID, "failed", "transcribe_failed", err.Error())
		mu.Lock()
		result.TranscribeErrors++
		mu.Unlock()
		return
	}
	if err := s.db.SetCallTranscript(ctx, call.ID, tr.Text, tr.Engine); err != nil {
		log.Printf("zvonari: saving transcript failed for call %s: %v", pbxUUID, err)
	}
}

// AnalyzeCalls finds every call in [from, to) whose transcript is ready but
// hasn't been classified yet, and runs the LLM outcome classification for
// each — decoupled from transcription (see transcribeOnly), so it can run
// on its own schedule (e.g. a few hours after the morning transcription
// batch) without either job blocking the other.
// analyzeBatchSize caps how many calls go into one call-analytics batch
// request. Larger batches mean fewer `hermes chat` subprocess round-trips
// (the actual bottleneck, not the classification itself), but also a bigger
// prompt/response and more to lose if the whole request errors. Under the
// IQ-200 v1.2 rubric each call's response is a full per-step breakdown
// (evidence/missing per step, not just {category, outcome, note}), which is
// far heavier to generate than the old flat classification — a batch of 50
// measured well over call_analytics_server.py's batch_timeout(50)=460s in
// practice, forcing a slow serial per-call fallback for the whole batch. 25
// is a deliberate middle ground: batch_timeout(25)=810s in
// call_analytics_server.py, plus up to ~25 more per-call fallback attempts
// in the worst case — see analyzeHTTPTimeout below, which is sized to
// outlast that worst case rather than abandon a batch that's still
// legitimately working.
const analyzeBatchSize = 25

// analyzeBatchCtxTimeout mirrors callreport.analyzeHTTPTimeout — kept as an
// independent context deadline (see the call site) rather than trusting the
// http.Client timeout alone.
const analyzeBatchCtxTimeout = 25 * time.Minute

func (s *Service) AnalyzeCalls(ctx context.Context, from, to time.Time) (*SyncResult, error) {
	if !s.callreport.Configured() {
		return nil, fmt.Errorf("аналитика недоступна (не настроен CALL_ANALYTICS_URL)")
	}

	calls, err := s.db.ListCallsNeedingAnalysis(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing calls needing analysis: %w", err)
	}

	result := &SyncResult{CallsFound: len(calls)}
	now := time.Now()
	s.syncMu.Lock()
	s.syncStatus.TotalToProcess = len(calls)
	s.syncStatus.Processed = 0
	s.stages["analyze"] = &StageStatus{State: StageRunning, Total: len(calls), StartedAt: &now}
	s.syncMu.Unlock()

	for start := 0; start < len(calls); start += analyzeBatchSize {
		s.waitIfPaused(ctx)
		// Stop() cancels ctx — don't start another batch once cancelled;
		// the in-flight one (if any) already aborted via batchCtx below.
		if ctx.Err() != nil {
			break
		}
		end := start + analyzeBatchSize
		if end > len(calls) {
			end = len(calls)
		}
		batch := calls[start:end]

		reqs := make([]callreport.AnalyzeCallRequest, len(batch))
		for i, c := range batch {
			reqs[i] = callreport.AnalyzeCallRequest{
				CallID:      c.ID,
				Transcript:  c.TranscriptText,
				DurationSec: c.DurationSec,
				TalkTimeSec: c.TalkTimeSec,
			}
		}

		// Belt-and-suspenders alongside callreport.Client's own httpClient.Timeout:
		// a real production run left a batch hung for 5+ hours with neither a
		// result nor an error ever coming back, well past that client timeout —
		// cause unconfirmed, but an explicit per-call context deadline is the
		// more standard Go mechanism and gives a second, independent way for
		// the request to actually get cancelled instead of relying on exactly
		// one timeout path that's already been observed not to fire.
		batchCtx, cancel := context.WithTimeout(ctx, analyzeBatchCtxTimeout)
		batchResult, err := s.callreport.AnalyzeCallsBatch(batchCtx, reqs)
		cancel()
		if err != nil {
			// Whole-batch failure (timeout, service down) — every call in it
			// stays without analytics_json, so the next AnalyzeCalls run
			// picks them all back up via ListCallsNeedingAnalysis.
			log.Printf("zvonari: batch analysis failed for %d calls: %v", len(batch), err)
			result.AnalyzeErrors += len(batch)
			s.syncMu.Lock()
			s.syncStatus.Processed += len(batch)
			s.stages["analyze"].Done += len(batch)
			s.syncMu.Unlock()
			continue
		}

		byID := make(map[string]json.RawMessage, len(batchResult.Results))
		for _, r := range batchResult.Results {
			byID[r.CallID] = r.AnalyticsJSON
		}
		for _, call := range batch {
			analytics, ok := byID[call.ID]
			if !ok || len(analytics) == 0 {
				log.Printf("zvonari: no analysis result for call %s in batch response", call.PBXUUID)
				result.AnalyzeErrors++
			} else if err := s.db.SetCallAnalytics(ctx, call.ID, analytics); err != nil {
				log.Printf("zvonari: saving analytics failed for call %s: %v", call.PBXUUID, err)
				result.AnalyzeErrors++
			}
			s.syncMu.Lock()
			s.syncStatus.Processed++
			s.stages["analyze"].Done++
			s.syncMu.Unlock()
		}
	}

	if ctx.Err() != nil {
		s.markStageFailed("analyze")
	} else {
		s.markStageDone("analyze", len(calls), len(calls))
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
// treatment SyncCalls does. Does NOT re-run analysis — call AnalyzeCall
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

// AnalyzeCall (re)runs Hermes analysis for one existing call, on demand from
// the UI — AnalyzeCalls' ListCallsNeedingAnalysis only ever picks up calls
// that don't already have analytics_json.call_type set, so a call that was
// already analyzed (e.g. before a prompt/rubric change, or before
// duration_sec/talk_time_sec started being sent — see the 2026-08 fraud-
// detection fix) had no way to be re-scored short of clearing its
// analytics_json by hand. Unlike RetranscribeCall this always re-analyzes
// regardless of current analytics_json state.
func (s *Service) AnalyzeCall(ctx context.Context, callID string) (*models.Call, error) {
	if !s.callreport.Configured() {
		return nil, fmt.Errorf("аналитика недоступна (не настроен CALL_ANALYTICS_URL)")
	}
	call, err := s.db.GetCallByID(ctx, callID)
	if err != nil {
		return nil, err
	}
	result, err := s.callreport.AnalyzeCall(ctx, callreport.AnalyzeCallRequest{
		CallID:      call.ID,
		Transcript:  call.TranscriptText,
		DurationSec: call.DurationSec,
		TalkTimeSec: call.TalkTimeSec,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzing call: %w", err)
	}
	if err := s.db.SetCallAnalytics(ctx, call.ID, result.AnalyticsJSON); err != nil {
		return nil, fmt.Errorf("saving analytics: %w", err)
	}
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

// GetOrGenerateReport reuses an existing report for the exact same
// caller+period if one is already in the DB, generating a fresh one via
// Hermes only when none exists — so an export/background read never
// re-pays the LLM cost for a period someone already analyzed. The explicit
// "Отчёт" button in the UI calls RequestCallerReport directly instead
// (always fresh), since a deliberate click implies "give me the current
// picture," not "reuse whatever's cached."
func (s *Service) GetOrGenerateReport(ctx context.Context, callerID, period string, from, to time.Time) (*models.CallerReport, error) {
	periodStart := from.Format("2006-01-02")
	periodEnd := to.Format("2006-01-02")
	if existing, err := s.db.GetLatestCallerReport(ctx, callerID, periodStart, periodEnd); err == nil && existing != nil {
		return existing, nil
	}
	return s.RequestCallerReport(ctx, callerID, period, from, to)
}

// ListReports returns a caller's past reports, most recent first — the
// history view so previously generated analyses stay reachable instead of
// only existing as rows nobody reads back.
func (s *Service) ListReports(ctx context.Context, callerID string, limit int) ([]models.CallerReport, error) {
	return s.db.ListCallerReports(ctx, callerID, limit)
}

// CallCounts returns how many synced calls each caller has in [from, to) —
// one query for all callers, for the caller-list UI to show a count badge
// per card without a request per caller.
func (s *Service) CallCounts(ctx context.Context, from, to time.Time) (map[string]int, error) {
	return s.db.CountCallsByCallerPeriod(ctx, from, to)
}

// ErrorBreakdown returns how many calls in [from, to) failed for each reason
// (error_kind), across every caller — задача 6, zvonari-improvements.md.
func (s *Service) ErrorBreakdown(ctx context.Context, from, to time.Time) (map[string]int, error) {
	return s.db.GetErrorBreakdown(ctx, from, to)
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

// OutcomeCounts returns each caller's Hermes outcome breakdown in [from, to)
// — one query for all callers, for the table view's per-caller conversion
// columns without an N+1 fetch.
func (s *Service) OutcomeCounts(ctx context.Context, from, to time.Time) (map[string]map[string]int, error) {
	return s.db.CountOutcomesByCaller(ctx, from, to)
}

// FraudCounts returns each caller's count of fraud_suspected calls
// (operator hit an answering machine/voicemail and didn't hang up promptly)
// in [from, to) — the time-padding-detection counterpart to OutcomeCounts,
// one query for all callers. A separate boolean axis from outcome, not one
// of its values — see CountFraudSuspectedByCaller.
func (s *Service) FraudCounts(ctx context.Context, from, to time.Time) (map[string]int, error) {
	return s.db.CountFraudSuspectedByCaller(ctx, from, to)
}

// CallDistribution buckets a caller's calls for a period by their Hermes
// outcome classification (analytics_json.outcome, one of the 13 closed-list
// values) — the UI's "brief distribution" view. No Hermes call needed: it
// aggregates analysis that already happened when each call was synced.
func (s *Service) CallDistribution(ctx context.Context, callerID string, from, to time.Time) (map[string]int, error) {
	calls, err := s.db.ListCallsByCallerPeriod(ctx, callerID, from, to)
	if err != nil {
		return nil, err
	}
	dist := map[string]int{}
	for _, c := range calls {
		dist[ExtractOutcome(c.AnalyticsJSON)]++
	}
	return dist, nil
}

func ExtractOutcome(raw json.RawMessage) string {
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

// ExtractCallType reads analytics_json.call_type ("технический" /
// "содержательный" / "недостаточно_данных") — the IQ-200 v1.2 rubric's
// classification of whether a real script-scorable conversation happened at
// all, independent of outcome and of fraud_suspected. Calls analyzed under
// the pre-rubric legacy {category, outcome} shape have no call_type and
// read back as "" here — callers must handle that the same way as
// "не проанализировано" rather than treating it as a fourth real value.
func ExtractCallType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var parsed struct {
		CallType string `json:"call_type"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	return parsed.CallType
}

// healthCacheTTL matches zvonari-improvements.md задача 7: a cached result
// no older than this is returned as-is instead of re-pinging every service
// on every poll from the header's status dots.
const healthCacheTTL = 30 * time.Second

// HealthStatus reports whether each external service the "Звонари" pipeline
// depends on is reachable — see задача 7: "если анализ падает, сразу видно,
// что это не приложение".
type HealthStatus struct {
	TranscribeCPU transcribe.PingResult `json:"transcribe_cpu"`
	TranscribeGPU transcribe.PingResult `json:"transcribe_gpu"`
	Analytics     callreport.PingResult `json:"analytics"`
	CheckedAt     time.Time             `json:"checked_at"`
}

// Health pings the CPU/GPU transcribe services and the call-analytics
// service, all three concurrently (each already bounded by its own 2s
// timeout), and caches the result for healthCacheTTL so the header's status
// dots don't hammer every configured service on every poll.
func (s *Service) Health(ctx context.Context) HealthStatus {
	s.healthMu.Lock()
	if s.healthCache != nil && time.Since(s.healthCachedAt) < healthCacheTTL {
		cached := *s.healthCache
		s.healthMu.Unlock()
		return cached
	}
	s.healthMu.Unlock()

	var wg sync.WaitGroup
	status := HealthStatus{CheckedAt: time.Now()}
	wg.Add(3)
	go func() { defer wg.Done(); status.TranscribeCPU = s.transcribe.PingCPU() }()
	go func() { defer wg.Done(); status.TranscribeGPU = s.transcribe.PingGPU() }()
	go func() { defer wg.Done(); status.Analytics = s.callreport.Ping(ctx) }()
	wg.Wait()

	s.healthMu.Lock()
	s.healthCache = &status
	s.healthCachedAt = status.CheckedAt
	s.healthMu.Unlock()
	return status
}

// ExtractFraudSuspected reads analytics_json.fraud_suspected — set when
// Hermes judged the call to be an answering machine/voicemail left open
// past when the operator should have hung up (suspected time-padding on the
// line), independent of call_type/outcome. Must match the fraud_suspected
// field in hermes/services/call_analytics_server.py's _normalize_analytics.
func ExtractFraudSuspected(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var parsed struct {
		FraudSuspected bool `json:"fraud_suspected"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false
	}
	return parsed.FraudSuspected
}
