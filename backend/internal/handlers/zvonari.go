package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
	"invoices-backend/internal/zvonari"
)

// parseRangeParams reads required `from`/`to` YYYY-MM-DD query params and
// returns a half-open [from, to) range with `to` inclusive of its whole day.
func parseRangeParams(r *http.Request) (time.Time, time.Time, error) {
	from, err := time.Parse("2006-01-02", r.URL.Query().Get("from"))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("параметр from обязателен и должен быть в формате YYYY-MM-DD")
	}
	to, err := time.Parse("2006-01-02", r.URL.Query().Get("to"))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("параметр to обязателен и должен быть в формате YYYY-MM-DD")
	}
	to = to.Add(24 * time.Hour)
	return from, to, nil
}

// GetCallers обрабатывает GET /api/zvonari/callers
func (h *Handlers) GetCallers(w http.ResponseWriter, r *http.Request) {
	callers, err := h.db.ListCallers(r.Context())
	if err != nil {
		log.Printf("GetCallers failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get callers")
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallerListResponse{Data: callers})
}

// SyncZvonariCalls обрабатывает POST /api/zvonari/sync?from=YYYY-MM-DD&to=YYYY-MM-DD
// Запускает синхронизацию в фоне и сразу возвращает ответ — сама синхронизация
// (скачивание записей + транскрибация + аналитика по каждому звонку) может
// занимать минуты, что не переживёт таймаут прокси/браузера, если держать
// HTTP-запрос открытым. Прогресс — через GET /api/zvonari/sync/status.
func (h *Handlers) SyncZvonariCalls(w http.ResponseWriter, r *http.Request) {
	if !h.zvonari.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "PBX не настроен (нет PBX_API_TOKEN/PBX_DOMAIN)")
		return
	}
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	started := h.zvonari.StartSync(from, to)
	if !started {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"data": map[string]string{"status": "already_running"},
		})
		return
	}
	respondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"data": map[string]string{"status": "started"},
	})
}

// GetZvonariSyncStatus обрабатывает GET /api/zvonari/sync/status
func (h *Handlers) GetZvonariSyncStatus(w http.ResponseWriter, r *http.Request) {
	status := h.zvonari.GetSyncStatus()
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": status})
}

// PauseZvonariSync обрабатывает POST /api/zvonari/sync/pause — приостанавливает
// текущую фоновую задачу (синк/повтор/анализ) перед следующим звонком/батчем,
// не теряя уже сделанный прогресс.
func (h *Handlers) PauseZvonariSync(w http.ResponseWriter, r *http.Request) {
	h.zvonari.Pause()
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": h.zvonari.GetSyncStatus()})
}

// ResumeZvonariSync обрабатывает POST /api/zvonari/sync/resume — снимает паузу,
// задача продолжает с того звонка/батча, на котором остановилась.
func (h *Handlers) ResumeZvonariSync(w http.ResponseWriter, r *http.Request) {
	h.zvonari.Resume()
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": h.zvonari.GetSyncStatus()})
}

// RetryFailedCalls обрабатывает POST /api/zvonari/calls/retry-failed?from=&to=&include_terminal=
// Массово (пере)запускает транскрибацию+аналитику для всех звонков за период
// с восстановимым статусом (failed/pending/transcribing) — работает в фоне,
// как и /sync, статус — тот же GET /api/zvonari/sync/status (это один и тот
// же "слот" фоновой задачи, синк и повтор не бегут одновременно).
// include_terminal=true дополнительно берёт no_recording — отдельным явным
// выбором, а не по умолчанию, так как повтор его не лечит (задача 6).
func (h *Handlers) RetryFailedCalls(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	includeTerminal := r.URL.Query().Get("include_terminal") == "true"
	started := h.zvonari.StartRetryFailed(from, to, includeTerminal)
	if !started {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"data": map[string]string{"status": "already_running"},
		})
		return
	}
	respondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"data": map[string]string{"status": "started"},
	})
}

// onlyCPUParam reads the `only_cpu` query flag shared by the GPU-retranscribe
// preview and start endpoints — defaults to true (the safer choice: skip
// calls already redone on GPU) so an old client/bookmark without the param
// doesn't accidentally fall back to a full-period re-run.
func onlyCPUParam(r *http.Request) bool {
	v := r.URL.Query().Get("only_cpu")
	return v != "false"
}

// GetRetranscribePreview обрабатывает GET /api/zvonari/calls/retranscribe-gpu/preview?from=&to=&only_cpu=
// Отдаёт числа для подтверждения перед запуском самой дорогой операции в
// системе: сколько звонков реально попадёт в перегон и сколько из них уже
// было на GPU, плюс грубую оценку длительности.
func (h *Handlers) GetRetranscribePreview(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	preview, err := h.zvonari.RetranscribePreview(r.Context(), from, to, onlyCPUParam(r))
	if err != nil {
		log.Printf("GetRetranscribePreview failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to build retranscribe preview")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": preview})
}

// RetranscribeAllCalls обрабатывает POST /api/zvonari/calls/retranscribe-gpu?from=&to=&only_cpu=
// Массово (пере)транскрибирует звонки за период — по умолчанию только те,
// что ещё не были на GPU (only_cpu=true), либо вообще все при only_cpu=false,
// включая уже готовые — в отличие от /retry-failed (только сломанные/
// необработанные). Транскрибация уже сама предпочитает GPU-бокс, если он
// настроен (TRANSCRIBE_SERVICE_GPU_URL) и доступен, иначе тихо падает на
// локальный CPU. Тот же общий "слот" фоновой задачи, что и у /sync,
// /retry-failed, /analyze.
func (h *Handlers) RetranscribeAllCalls(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	started := h.zvonari.StartRetranscribeAll(from, to, onlyCPUParam(r))
	if !started {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"data": map[string]string{"status": "already_running"},
		})
		return
	}
	respondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"data": map[string]string{"status": "started"},
	})
}

// AnalyzeCalls обрабатывает POST /api/zvonari/calls/analyze?from=&to=
// Запускает LLM-классификацию для звонков за период, у которых уже есть
// готовый транскрипт, но ещё нет analytics_json — отдельная фоновая задача
// от транскрибации (см. transcribeOnly в service.go): может запускаться
// с задержкой, например через несколько часов после утреннего синка.
func (h *Handlers) AnalyzeCalls(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	started := h.zvonari.StartAnalyzeCalls(from, to)
	if !started {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"data": map[string]string{"status": "already_running"},
		})
		return
	}
	respondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"data": map[string]string{"status": "started"},
	})
}

// GetZvonariHealth обрабатывает GET /api/zvonari/health — пингует
// hermes_call_transcribe (CPU и, если настроен, GPU-бокс) и
// hermes_call_analytics с таймаутом 2с и кешем на 30с, чтобы в шапке можно
// было сразу показать, что не отвечает, вместо того чтобы гадать по ошибке
// синхронизации/анализа (задача 7, zvonari-improvements.md).
func (h *Handlers) GetZvonariHealth(w http.ResponseWriter, r *http.Request) {
	status := h.zvonari.Health(r.Context())
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": status})
}

// GetErrorBreakdown обрабатывает GET /api/zvonari/calls/error-breakdown?from=&to=
// Разбивка неудачных звонков по причине (error_kind) за период, по всем
// звонарям — сводка под блоком синхронизации вместо одной красной строки
// "ошибка" (задача 6, zvonari-improvements.md).
func (h *Handlers) GetErrorBreakdown(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	counts, err := h.zvonari.ErrorBreakdown(r.Context(), from, to)
	if err != nil {
		log.Printf("GetErrorBreakdown failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get error breakdown")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": counts})
}

// GetCallStatusCounts обрабатывает GET /api/zvonari/calls/status-counts?from=&to=
// Разбивка звонков по transcript_status на каждого звонаря за период —
// сколько реально готово/упало/без записи/в очереди, а не просто общий счётчик.
func (h *Handlers) GetCallStatusCounts(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	counts, err := h.zvonari.CallStatusCounts(r.Context(), from, to)
	if err != nil {
		log.Printf("GetCallStatusCounts failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get call status counts")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": counts})
}

// GetCallCounts обрабатывает GET /api/zvonari/calls/count?from=&to=
// Возвращает число синхронизированных звонков за период по каждому звонарю
// (map caller_id -> count) — для счётчика на карточках в списке.
func (h *Handlers) GetCallCounts(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	counts, err := h.zvonari.CallCounts(r.Context(), from, to)
	if err != nil {
		log.Printf("GetCallCounts failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get call counts")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": counts})
}

// GetOutcomeCounts обрабатывает GET /api/zvonari/calls/outcomes?from=&to=
// Разбивка по итогам оценки скрипта IQ-200 (регламент v1.2, закрытый список
// из 13 значений — "Скрипт пройден до шага 6", "Срыв на шаге 1" и т.д.) на
// каждого звонаря за период — для сортировки/подсветки в таблице звонарей
// без отдельного запроса на каждого.
func (h *Handlers) GetOutcomeCounts(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	counts, err := h.zvonari.OutcomeCounts(r.Context(), from, to)
	if err != nil {
		log.Printf("GetOutcomeCounts failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get outcome counts")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": counts})
}

// GetFraudCounts обрабатывает GET /api/zvonari/calls/fraud-counts?from=&to=
// Число звонков с fraud_suspected=true (не сброшенный вовремя автоответчик)
// на каждого звонаря за период — для выявления АФК-прослушивания
// автоответчика без отдельного запроса на каждого звонаря.
func (h *Handlers) GetFraudCounts(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	counts, err := h.zvonari.FraudCounts(r.Context(), from, to)
	if err != nil {
		log.Printf("GetFraudCounts failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get fraud counts")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": counts})
}

// GetCallerCalls обрабатывает GET /api/zvonari/callers/{id}/calls?from=&to=
// Детализация звонков звонаря за период: время, направление, длительность,
// полный разбор по шагам скрипта (analytics_json) и транскрипт по каждому звонку.
func (h *Handlers) GetCallerCalls(w http.ResponseWriter, r *http.Request) {
	callerID := chi.URLParam(r, "id")
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	calls, err := h.zvonari.ListCalls(r.Context(), callerID, from, to)
	if err != nil {
		log.Printf("GetCallerCalls failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get calls")
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallListResponse{Data: calls})
}

// RetranscribeCall обрабатывает POST /api/zvonari/calls/{id}/transcribe
// Ручной (пере)запуск транскрибации+аналитики для одного звонка — например,
// если он застрял в статусе "transcribing"/"failed" после перезапуска
// бэкенда посреди синка. Синхронный: один звонок занимает секунды, не минуты.
func (h *Handlers) RetranscribeCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "id")

	call, err := h.zvonari.RetranscribeCall(r.Context(), callID)
	if err != nil {
		log.Printf("RetranscribeCall failed: %v", err)
		respondNotFoundOrInternal(w, err, "Call not found")
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallResponse{Data: *call})
}

// AnalyzeCall обрабатывает POST /api/zvonari/calls/{id}/analyze
// Ручной (пере)запуск только аналитики Hermes для одного звонка — в отличие
// от RetranscribeCall не трогает транскрипт, только пересчитывает
// classification/fraud_suspected. Нужен, когда звонок уже был проанализирован
// раньше (regular AnalyzeCalls пропускает уже размеченные звонки) — например
// после смены регламента/промпта на стороне Hermes.
func (h *Handlers) AnalyzeCall(w http.ResponseWriter, r *http.Request) {
	callID := chi.URLParam(r, "id")

	call, err := h.zvonari.AnalyzeCall(r.Context(), callID)
	if err != nil {
		log.Printf("AnalyzeCall failed: %v", err)
		respondNotFoundOrInternal(w, err, "Call not found")
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallResponse{Data: *call})
}

// GetCallerCallDistribution обрабатывает GET /api/zvonari/callers/{id}/distribution?from=&to=
func (h *Handlers) GetCallerCallDistribution(w http.ResponseWriter, r *http.Request) {
	callerID := chi.URLParam(r, "id")
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	dist, err := h.zvonari.CallDistribution(r.Context(), callerID, from, to)
	if err != nil {
		log.Printf("GetCallerCallDistribution failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get distribution")
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallDistributionResponse{Data: dist})
}

// RequestCallerReport обрабатывает POST /api/zvonari/callers/{id}/report
func (h *Handlers) RequestCallerReport(w http.ResponseWriter, r *http.Request) {
	callerID := chi.URLParam(r, "id")

	var req models.RequestCallerReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	from, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный from")
		return
	}
	to, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный to")
		return
	}
	to = to.Add(24 * time.Hour)

	report, err := h.zvonari.RequestCallerReport(r.Context(), callerID, req.Period, from, to)
	if err != nil {
		log.Printf("RequestCallerReport failed: %v", err)
		respondWithError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallerReportResponse{Data: *report})
}

// GetCallerReportHistory обрабатывает GET /api/zvonari/callers/{id}/reports?limit=
// Прошлые сгенерированные отчёты по звонарю, новые сверху — чтобы уже
// оплаченный/сформированный анализ оставался доступен, а не терялся
// после того, как страницу перезагрузили.
func (h *Handlers) GetCallerReportHistory(w http.ResponseWriter, r *http.Request) {
	callerID := chi.URLParam(r, "id")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	reports, err := h.zvonari.ListReports(r.Context(), callerID, limit)
	if err != nil {
		log.Printf("GetCallerReportHistory failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get report history")
		return
	}
	respondWithJSON(w, http.StatusOK, models.CallerReportListResponse{Data: reports})
}

// ExportCallerCallsCSV обрабатывает GET /api/zvonari/callers/{id}/export.csv?from=&to=
// CSV со звонками звонаря за период (дата, время, направление, длительность,
// категория, транскрипт) плюс общая оценка за этот период в последней
// колонке на каждой строке — использует уже сохранённый в БД отчёт для
// точно этого периода, если такой есть, и генерирует новый только если
// ещё не запрашивался (см. Service.GetOrGenerateReport).
func (h *Handlers) ExportCallerCallsCSV(w http.ResponseWriter, r *http.Request) {
	callerID := chi.URLParam(r, "id")
	from, to, err := parseRangeParams(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	caller, err := h.db.GetCallerByID(r.Context(), callerID)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Caller not found")
		return
	}

	calls, err := h.zvonari.ListCalls(r.Context(), callerID, from, to)
	if err != nil {
		log.Printf("ExportCallerCallsCSV: listing calls failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to load calls")
		return
	}

	var summaryText string
	if report, err := h.zvonari.GetOrGenerateReport(r.Context(), callerID, "custom", from, to); err != nil {
		// Экспорт не должен падать целиком из-за недоступной аналитики —
		// звонки со всё равно ценны сами по себе, просто без итоговой оценки.
		log.Printf("ExportCallerCallsCSV: report unavailable, exporting without it: %v", err)
	} else {
		summaryText = report.SummaryText
	}

	filename := fmt.Sprintf("zvonari_%s_%s_%s.csv", caller.PBXExtension, from.Format("2006-01-02"), to.Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	// UTF-8 BOM so Excel on Windows renders Cyrillic correctly instead of mojibake.
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Дата", "Время", "Направление", "Длительность звонка (сек)", "Время разговора (сек)", "Тип звонка", "Исход", "Фрод (не сброшен автоответчик)", "Транскрипт", "Общая оценка за период"})
	for _, c := range calls {
		fraud := ""
		if zvonari.ExtractFraudSuspected(c.AnalyticsJSON) {
			fraud = "да"
		}
		_ = cw.Write([]string{
			c.StartedAt.Format("2006-01-02"),
			c.StartedAt.Format("15:04:05"),
			c.Direction,
			strconv.Itoa(c.DurationSec),
			strconv.Itoa(c.TalkTimeSec),
			zvonari.ExtractCallType(c.AnalyticsJSON),
			zvonari.ExtractOutcome(c.AnalyticsJSON),
			fraud,
			c.TranscriptText,
			summaryText,
		})
	}
	cw.Flush()
}
