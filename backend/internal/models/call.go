package models

import (
	"encoding/json"
	"time"
)

// Call представляет звонок, синхронизированный из CDR OnlinePBX
type Call struct {
	ID                 string          `json:"id"`
	PBXUUID            string          `json:"pbx_uuid"`
	CallerID           *string         `json:"caller_id"`
	Direction          string          `json:"direction"`
	CounterpartyNumber string          `json:"counterparty_number"`
	StartedAt          time.Time       `json:"started_at"`
	DurationSec        int             `json:"duration_sec"`
	TalkTimeSec        int             `json:"talk_time_sec"`
	HangupCause        string          `json:"hangup_cause"`
	TranscriptStatus   string          `json:"transcript_status"`
	TranscriptText     string          `json:"transcript_text,omitempty"`
	AnalyticsJSON      json.RawMessage `json:"analytics_json,omitempty"`
	// Engine — какой движок фактически транскрибировал этот звонок
	// ("cpu" / "gpu"), пусто пока звонок ещё не транскрибирован.
	Engine        string     `json:"engine,omitempty"`
	TranscribedAt *time.Time `json:"transcribed_at,omitempty"`
	// ErrorKind/LastError — причина последней ошибки транскрибации
	// (no_recording/download_failed/transcribe_failed), пусто если звонок
	// готов или ещё не обрабатывался. Успешная (пере)транскрибация чистит
	// оба поля — см. SetCallTranscript.
	ErrorKind string    `json:"error_kind,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// FraudPrecheck* — фрод-сигнал, определённый на этапе транскрибации по
	// аудио (см. transcribe.Client.transcribeOpenRouter), отдельно от
	// analytics_json (тот пишет анализ по критериям, не связанным с фродом).
	// FraudPrecheckSuspected == nil значит "предфильтр не выполнялся вообще"
	// (GPU/CPU транскрибация, звонок ещё не обработан) — это НЕ то же самое,
	// что false ("выполнялся, фрода не нашёл"), см. SetCallTranscript.
	FraudPrecheckSuspected *bool  `json:"fraud_precheck_suspected,omitempty"`
	FraudPrecheckCheck     string `json:"fraud_precheck_check,omitempty"`
	FraudPrecheckQuote     string `json:"fraud_precheck_quote,omitempty"`
	FraudPrecheckPosition  string `json:"fraud_precheck_position,omitempty"`
	// FraudReviewStatus — "pending" (ждёт решения человека) / "confirmed" /
	// "rejected", пусто если звонок не был кандидатом на фрод вообще.
	// Единственный источник, влияющий на фактическое исключение из оплаты —
	// это внешний расчёт, который смотрит "confirmed"; сам по себе
	// FraudPrecheckSuspected ничего не исключает, см. миграцию 034.
	FraudReviewStatus string     `json:"fraud_review_status,omitempty"`
	FraudReviewedAt   *time.Time `json:"fraud_reviewed_at,omitempty"`
}

// CallListResponse представляет ответ со списком звонков
type CallListResponse struct {
	Data []Call `json:"data"`
}

// CallResponse представляет ответ с одним звонком
type CallResponse struct {
	Data Call `json:"data"`
}

// CallerReport представляет сводный отчёт Hermes по звонарю за период
type CallerReport struct {
	ID          string          `json:"id"`
	CallerID    string          `json:"caller_id"`
	Period      string          `json:"period"`
	PeriodStart string          `json:"period_start"`
	PeriodEnd   string          `json:"period_end"`
	SummaryText string          `json:"summary_text"`
	MetricsJSON json.RawMessage `json:"metrics_json,omitempty"`
	RequestedAt time.Time       `json:"requested_at"`
}

// CallerReportResponse представляет ответ с отчётом по звонарю
type CallerReportResponse struct {
	Data CallerReport `json:"data"`
}

// CallerReportListResponse представляет ответ со списком прошлых отчётов по звонарю
type CallerReportListResponse struct {
	Data []CallerReport `json:"data"`
}

// CallDistributionResponse представляет распределение звонков по категориям
// (аналитика от Hermes) за запрошенный период
type CallDistributionResponse struct {
	Data map[string]int `json:"data"`
}

// SyncCallsResult представляет итог синхронизации звонков с OnlinePBX
type SyncCallsResult struct {
	CallersSynced    int `json:"callers_synced"`
	CallsFound       int `json:"calls_found"`
	CallsNew         int `json:"calls_new"`
	CallsSkipped     int `json:"calls_skipped"`
	TranscribeErrors int `json:"transcribe_errors"`
}

// SyncCallsResponse представляет ответ на запуск синхронизации
type SyncCallsResponse struct {
	Data SyncCallsResult `json:"data"`
}

// RequestCallerReportRequest представляет запрос на генерацию отчёта по звонарю
type RequestCallerReportRequest struct {
	Period string `json:"period"` // "day" | "week" | "custom"
	From   string `json:"from"`
	To     string `json:"to"`
}
