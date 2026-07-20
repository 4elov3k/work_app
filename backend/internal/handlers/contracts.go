package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
)

// GetContracts обрабатывает GET /api/contracts
func (h *Handlers) GetContracts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	customerID := r.URL.Query().Get("customer_id")
	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	page := 1
	perPage := 100

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 {
			perPage = pp
		}
	}

	contracts, total, err := h.db.GetContracts(ctx, customerID, page, perPage)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get contracts")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ContractListResponse{
		Data:    contracts,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// GetContractByID обрабатывает GET /api/contracts/{id}
func (h *Handlers) GetContractByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	contract, err := h.db.GetContractByID(ctx, id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Contract not found")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ContractResponse{Data: *contract})
}

// CreateContract обрабатывает POST /api/contracts
func (h *Handlers) CreateContract(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CustomerID == "" {
		respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}
	if req.Number == "" {
		respondWithError(w, http.StatusBadRequest, "Contract number is required")
		return
	}
	if !isDigitsOnly(req.Number) {
		respondWithError(w, http.StatusBadRequest, "Contract number must be numeric")
		return
	}

	if req.Topic == "" {
		respondWithError(w, http.StatusBadRequest, "Contract topic is required")
		return
	}

	if req.Currency == "" {
		req.Currency = "RUB"
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Status == "archive" {
		req.Status = "archived"
	}

	allowedStatus := map[string]bool{
		"active":   true,
		"archived": true,
		"draft":    true,
		"closed":   true,
		"canceled": true,
	}
	if !allowedStatus[req.Status] {
		respondWithError(w, http.StatusBadRequest, "Invalid contract status")
		return
	}

	allowedTopics := map[string]bool{
		"Продвижение сео":      true,
		"Продвижение контекст": true,
		"Сео + контекст":       true,
		"Техподдержка":         true,
		"Юр услуги":            true,
		"Разработка":           true,
		"Соц сети":             true,
		"Дизайн":               true,
		"Отзывы":               true,
	}
	if !allowedTopics[req.Topic] {
		respondWithError(w, http.StatusBadRequest, "Invalid contract topic")
		return
	}

	if req.StartDate != "" {
		if _, err := time.Parse("2006-01-02", req.StartDate); err != nil {
			if _, errAlt := time.Parse("02.01.2006", req.StartDate); errAlt != nil {
				respondWithError(w, http.StatusBadRequest, "Invalid start_date format")
				return
			}
			req.StartDate = swapDateFormat(req.StartDate)
		}
	}
	if req.EndDate != "" {
		if _, err := time.Parse("2006-01-02", req.EndDate); err != nil {
			if _, errAlt := time.Parse("02.01.2006", req.EndDate); errAlt != nil {
				respondWithError(w, http.StatusBadRequest, "Invalid end_date format")
				return
			}
			req.EndDate = swapDateFormat(req.EndDate)
		}
	}

	contract, err := h.db.CreateContract(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Contract with this number already exists for this customer")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to create contract")
		return
	}

	respondWithJSON(w, http.StatusCreated, models.ContractResponse{Data: *contract})
}

// DeleteContract обрабатывает DELETE /api/contracts/{id}
func (h *Handlers) DeleteContract(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	if err := h.db.DeleteContract(ctx, id); err != nil {
		if isForeignKeyViolation(err) {
			respondWithError(w, http.StatusConflict, "Contract has linked documents and cannot be deleted")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Contract not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to delete contract")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func swapDateFormat(ddmmyyyy string) string {
	parts := strings.Split(ddmmyyyy, ".")
	if len(parts) != 3 {
		return ddmmyyyy
	}
	return parts[2] + "-" + parts[1] + "-" + parts[0]
}

// GetNextContractNumber обрабатывает GET /api/contracts/next?customer_id=...
func (h *Handlers) GetNextContractNumber(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	next, err := h.db.GetNextContractNumber(ctx, customerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get next contract number")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"number": strconv.FormatInt(next, 10)})
}

// GetNextContractDocumentNumber обрабатывает GET /api/contracts/{id}/next-number?type=invoice|act
func (h *Handlers) GetNextContractDocumentNumber(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	docType := r.URL.Query().Get("type")
	if docType != "invoice" && docType != "act" {
		respondWithError(w, http.StatusBadRequest, "Invalid type")
		return
	}

	var next int64
	var err error
	if docType == "invoice" {
		next, err = h.db.GetNextInvoiceNumber(ctx, contractID)
	} else {
		next, err = h.db.GetNextActNumber(ctx, contractID)
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get next number")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"number": strconv.FormatInt(next, 10)})
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func digitsOnly(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
