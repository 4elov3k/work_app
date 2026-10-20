package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// GetGoogleAuthStatus обрабатывает GET /api/google/status
// Показывает, жив ли Google-токен sheets-sync (через него идут номера
// счетов/актов из таблиц).
func (h *Handlers) GetGoogleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}
	status, err := h.sheetsync.GoogleStatus(r.Context())
	if err != nil {
		log.Printf("sheetsync: google status failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Не удалось получить статус Google-авторизации")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": status})
}

// StartGoogleAuth обрабатывает POST /api/google/auth-url
// Возвращает ссылку на экран согласия Google. После согласия браузер уйдёт на
// http://localhost:1/?code=… (страница не откроется — так и задумано), и этот
// адрес нужно вставить обратно в интерфейс.
func (h *Handlers) StartGoogleAuth(w http.ResponseWriter, r *http.Request) {
	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}
	url, err := h.sheetsync.GoogleAuthURL(r.Context())
	if err != nil {
		log.Printf("sheetsync: google auth-url failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Не удалось получить ссылку авторизации")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]string{"url": url}})
}

// FinishGoogleAuth обрабатывает POST /api/google/auth-code
// Принимает вставленный адрес из браузера (или голый code) и меняет его на токен.
func (h *Handlers) FinishGoogleAuth(w http.ResponseWriter, r *http.Request) {
	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		respondWithError(w, http.StatusBadRequest, "Вставьте адрес из браузера после входа в Google")
		return
	}
	status, err := h.sheetsync.GoogleAuthCode(r.Context(), strings.TrimSpace(req.Code))
	if err != nil {
		log.Printf("sheetsync: google auth-code failed: %v", err)
		msg := "Не удалось подключить Google. Получите новую ссылку и попробуйте ещё раз"
		if strings.Contains(err.Error(), "state mismatch") || strings.Contains(err.Error(), "no pending") {
			msg = "Ссылка устарела или уже использована — получите новую"
		}
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"data": status})
}
