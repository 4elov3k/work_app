package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
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

// GetCallerCalls обрабатывает GET /api/zvonari/callers/{id}/calls?from=&to=
// Детализация звонков звонаря за период: время, направление, длительность,
// категория (analytics_json.outcome) и транскрипт по каждому звонку.
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
