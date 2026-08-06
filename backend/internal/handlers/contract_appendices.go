package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
)

// GetNextContractAppendixNumber обрабатывает GET /api/contracts/{id}/appendices/next-number
func (h *Handlers) GetNextContractAppendixNumber(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	next, err := h.db.GetNextContractAppendixNumber(ctx, contractID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get next appendix number")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"number": strconv.FormatInt(next, 10)})
}

// GetContractAppendices обрабатывает GET /api/contracts/{id}/appendices
func (h *Handlers) GetContractAppendices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	appendices, err := h.db.GetContractAppendicesByContract(ctx, contractID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get contract appendices")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ContractAppendixListResponse{
		Data:    appendices,
		Total:   len(appendices),
		Page:    1,
		PerPage: len(appendices),
	})
}

// CreateContractAppendix обрабатывает POST /api/contracts/{id}/appendices
func (h *Handlers) CreateContractAppendix(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	var req models.CreateContractAppendixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ContractID = contractID

	if req.Number == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix number is required")
		return
	}
	if !isDigitsOnly(req.Number) {
		respondWithError(w, http.StatusBadRequest, "Appendix number must be numeric")
		return
	}
	if req.Date == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix date is required")
		return
	}
	if _, err := time.Parse("02.01.2006", req.Date); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid appendix date format")
		return
	}
	if len(req.Lines) == 0 {
		respondWithError(w, http.StatusBadRequest, "At least one line is required")
		return
	}

	appendix, err := h.db.CreateContractAppendix(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Appendix with this number already exists for this contract")
			return
		}
		if isRecordNotFoundError(err) {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, models.ContractAppendixWithLinesResponse{Data: *appendix})
}

// GetContractAppendix обрабатывает GET /api/contract-appendices/{id}
func (h *Handlers) GetContractAppendix(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix ID is required")
		return
	}

	appendix, err := h.db.GetContractAppendixWithLines(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Contract appendix not found")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ContractAppendixWithLinesResponse{Data: *appendix})
}

// UpdateContractAppendix обрабатывает PATCH /api/contract-appendices/{id}
func (h *Handlers) UpdateContractAppendix(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix ID is required")
		return
	}

	var req models.UpdateContractAppendixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Number != nil && !isDigitsOnly(*req.Number) {
		respondWithError(w, http.StatusBadRequest, "Appendix number must be numeric")
		return
	}
	if req.Date != nil {
		if _, err := time.Parse("02.01.2006", *req.Date); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid appendix date format")
			return
		}
	}

	appendix, err := h.db.UpdateContractAppendix(ctx, id, req)
	if err != nil {
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Appendix with this number already exists for this contract")
			return
		}
		respondNotFoundOrInternal(w, err, "Contract appendix not found")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ContractAppendixResponse{Data: *appendix})
}

// DeleteContractAppendix обрабатывает DELETE /api/contract-appendices/{id}
func (h *Handlers) DeleteContractAppendix(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix ID is required")
		return
	}

	if err := h.db.DeleteContractAppendix(ctx, id); err != nil {
		if isForeignKeyViolation(err) {
			respondWithError(w, http.StatusConflict, "Appendix is in use and cannot be deleted")
			return
		}
		respondNotFoundOrInternal(w, err, "Contract appendix not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AddContractAppendixLine обрабатывает POST /api/contract-appendices/{id}/lines
func (h *Handlers) AddContractAppendixLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix ID is required")
		return
	}

	var req models.AddContractAppendixLineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.db.AddContractAppendixLine(ctx, id, req.Line); err != nil {
		if isRecordNotFoundError(err) {
			respondWithError(w, http.StatusNotFound, "Contract appendix not found")
			return
		}
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteContractAppendixLine обрабатывает DELETE /api/contract-appendices/{id}/lines/{lineID}
func (h *Handlers) DeleteContractAppendixLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		respondWithError(w, http.StatusBadRequest, "Appendix ID and line ID are required")
		return
	}

	if err := h.db.DeleteContractAppendixLine(ctx, id, lineID); err != nil {
		respondNotFoundOrInternal(w, err, "Appendix line not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
