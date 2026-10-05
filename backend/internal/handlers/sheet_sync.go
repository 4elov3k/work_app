package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// validInvoiceCategories — the three fixed project buckets a human picks in
// the UI for the invoice registry's "Наименование" column. An invoice's
// actual service line items don't map 1:1 onto one of these (an invoice can
// span several services), so this is a manual classification, not derived
// from the invoice data. sheets-sync validates this again server-side, but
// work_app rejects garbage before it ever leaves this process.
var validInvoiceCategories = map[string]bool{
	"Продвижение": true,
	"Разработка":  true,
	"Хостинг":     true,
}

// GetActNumberFromSheet обрабатывает GET /api/acts/next-number-from-sheet
// Возвращает следующий номер акта из внешнего реестра (Google-таблицы),
// а не из внутренней последовательности work_app — по явному требованию,
// что номер в приложении должен совпадать с номером в реальном журнале.
func (h *Handlers) GetActNumberFromSheet(w http.ResponseWriter, r *http.Request) {
	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}

	result, err := h.sheetsync.NextActNumber(r.Context())
	if err != nil {
		log.Printf("sheetsync: next-act-number failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Не удалось получить номер из таблицы")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"number": result.Number,
			"row":    result.Row,
		},
	})
}

// RegisterActInSheet обрабатывает POST /api/acts/{id}/register-in-sheet
// Дописывает строку в реестр для уже созданного акта. Номер и строка
// вычисляются заново на стороне sheets-sync в момент записи — не
// переиспользуют то, что вернул next-number-from-sheet ранее, чтобы не
// записать устаревшее значение.
func (h *Handlers) RegisterActInSheet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}

	act, err := h.db.GetActByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Act not found")
		return
	}

	customer, err := h.db.GetCustomerByID(ctx, act.CustomerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load customer for sheet registration")
		return
	}

	result, err := h.sheetsync.RegisterAct(ctx, customer.Name, act.Date)
	if err != nil {
		log.Printf("sheetsync: register act %s failed: %v", id, err)
		respondWithError(w, http.StatusBadGateway, "Не удалось зарегистрировать акт в таблице")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"row":           result.Row,
			"number":        result.Number,
			"updated_cells": result.UpdatedCells,
		},
	})
}

// GetInvoiceNumberFromSheet обрабатывает GET /api/invoices/next-number-from-sheet
// Возвращает следующий номер счета из внешнего реестра (Google-таблицы) —
// см. GetActNumberFromSheet выше, тот же принцип, отдельная таблица.
func (h *Handlers) GetInvoiceNumberFromSheet(w http.ResponseWriter, r *http.Request) {
	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}

	result, err := h.sheetsync.NextInvoiceNumber(r.Context())
	if err != nil {
		log.Printf("sheetsync: next-invoice-number failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Не удалось получить номер из таблицы")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"number": result.Number,
			"row":    result.Row,
		},
	})
}

// RegisterInvoiceInSheet обрабатывает POST /api/invoices/{id}/register-in-sheet
// Дописывает строку в реестр для уже созданного счета. Category — одна из
// трёх фиксированных категорий проекта ("Продвижение"/"Разработка"/
// "Хостинг"), выбранная человеком в UI, не выводится автоматически из
// состава услуг счета.
func (h *Handlers) RegisterInvoiceInSheet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	var req struct {
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if !validInvoiceCategories[req.Category] {
		respondWithError(w, http.StatusBadRequest, "category должна быть одной из: Продвижение, Разработка, Хостинг")
		return
	}

	if !h.sheetsync.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Синхронизация с таблицей недоступна (не настроен SHEETS_SYNC_URL)")
		return
	}

	invoice, err := h.db.GetInvoiceByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Invoice not found")
		return
	}

	customer, err := h.db.GetCustomerByID(ctx, invoice.CustomerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load customer for sheet registration")
		return
	}

	result, err := h.sheetsync.RegisterInvoice(ctx, customer.Fullname, customer.Name, req.Category, invoice.Date)
	if err != nil {
		log.Printf("sheetsync: register invoice %s failed: %v", id, err)
		respondWithError(w, http.StatusBadGateway, "Не удалось зарегистрировать счет в таблице")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"row":           result.Row,
			"number":        result.Number,
			"updated_cells": result.UpdatedCells,
		},
	})
}
