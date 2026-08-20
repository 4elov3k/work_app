package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/export/updxml"
	"invoices-backend/internal/models"
	"invoices-backend/internal/saby"
)

// GetActs обрабатывает GET /api/acts
func (h *Handlers) GetActs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	customerID := r.URL.Query().Get("customer_id")
	contractID := r.URL.Query().Get("contract_id")
	archivedStr := r.URL.Query().Get("archived")
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

	var archived *bool
	if archivedStr != "" {
		value := archivedStr == "true" || archivedStr == "1"
		archived = &value
	}

	acts, total, err := h.db.GetActs(ctx, customerID, contractID, archived, page, perPage)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get acts")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ActListResponse{
		Data:    acts,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// GetActByID обрабатывает GET /api/acts/{id}
func (h *Handlers) GetActByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	act, err := h.db.GetActByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Act not found")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ActResponse{Data: *act})
}

// GetActWithServices обрабатывает GET /api/acts/{id}/services
func (h *Handlers) GetActWithServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	act, err := h.db.GetActWithServices(ctx, id)
	if err != nil {
		log.Printf("Error getting act with services (ID: %s): %v", id, err)
		respondNotFoundOrInternal(w, err, "Act not found")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ActWithServicesResponse{Data: *act})
}

// ExportActUPDXML обрабатывает GET /api/acts/{id}/export/upd-xml
func (h *Handlers) ExportActUPDXML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	act, err := h.db.GetActWithServices(ctx, id)
	if err != nil {
		log.Printf("Error exporting act XML (ID: %s): %v", id, err)
		respondNotFoundOrInternal(w, err, "Act not found")
		return
	}

	customer, err := h.db.GetCustomerByID(ctx, act.CustomerID)
	if err != nil {
		respondLinkedRecordOrInternal(w, err, "Customer not found for act", fmt.Sprintf("Error loading customer for act export (act ID: %s)", id))
		return
	}

	contract, err := h.db.GetContractByID(ctx, act.ContractID)
	if err != nil {
		respondLinkedRecordOrInternal(w, err, "Contract not found for act", fmt.Sprintf("Error loading contract for act export (act ID: %s)", id))
		return
	}

	sellerEDOID, err := h.resolveActEDOParticipants(ctx, customer)
	if err != nil {
		log.Printf("Error resolving act EDO participants (act ID: %s, customer ID: %s): %v", id, customer.ID, err)
		respondWithError(w, http.StatusBadGateway, err.Error())
		return
	}

	org, err := h.db.GetActiveOrganization(ctx)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Organization is not configured")
		return
	}

	data, filename, err := updxml.BuildActUPDXMLWithSellerEDOID(*act, *customer, *contract, updxml.SellerFromOrganization(*org), sellerEDOID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="act.xml"; filename*=UTF-8''`+url.PathEscape(filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handlers) resolveActEDOParticipants(ctx context.Context, customer *models.Customer) (string, error) {
	sellerEDOID := strings.TrimSpace(os.Getenv("SABY_SELLER_EDO_ID"))
	if h.saby == nil || !h.saby.Enabled() {
		return sellerEDOID, nil
	}

	cachedCustomerEDOID := strings.TrimSpace(customer.EDOIDTensor)
	edoID, err := h.saby.LookupParticipantID(ctx, saby.Party{
		INN:  customer.INN,
		KPP:  customer.KPP,
		Name: customerNameForLookup(*customer),
	})
	if err != nil {
		if cachedCustomerEDOID == "" {
			return "", fmt.Errorf("не удалось получить идентификатор ЭДО покупателя через Saby (СБИС) и нет сохранённого значения: %w", err)
		}
		log.Printf("Failed to refresh customer Saby EDO ID, using cached value (customer ID: %s): %v", customer.ID, err)
	} else {
		customer.EDOIDTensor = edoID
		if edoID != cachedCustomerEDOID {
			if err := h.db.UpdateCustomerTensorEDOID(ctx, customer.ID, edoID); err != nil {
				log.Printf("Failed to cache customer Saby EDO ID (customer ID: %s): %v", customer.ID, err)
			}
		}
	}

	if sellerEDOID == "" {
		edoID, err = h.saby.LookupParticipantID(ctx, saby.Party{
			INN:  getenvDefault("SABY_SELLER_INN", "526220116209"),
			KPP:  os.Getenv("SABY_SELLER_KPP"),
			Name: os.Getenv("SABY_SELLER_NAME"),
		})
		if err != nil {
			if cached := strings.TrimSpace(os.Getenv("SABY_SELLER_EDO_ID")); cached != "" {
				return cached, nil
			}
			return "", fmt.Errorf("не удалось получить идентификатор ЭДО продавца через Saby (СБИС), и SABY_SELLER_EDO_ID не задан вручную: %w", err)
		}
		sellerEDOID = edoID
	}

	return sellerEDOID, nil
}

func customerNameForLookup(customer models.Customer) string {
	if strings.TrimSpace(customer.Fullname) != "" {
		return customer.Fullname
	}
	return customer.Name
}

func getenvDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	return fallback
}

// CreateAct обрабатывает POST /api/acts
func (h *Handlers) CreateAct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateActRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Number == "" {
		respondWithError(w, http.StatusBadRequest, "Act number is required")
		return
	}
	if !isDigitsOnly(req.Number) {
		respondWithError(w, http.StatusBadRequest, "Act number must be numeric")
		return
	}
	if req.Date == "" {
		respondWithError(w, http.StatusBadRequest, "Act date is required")
		return
	}
	if len(req.ServiceIDs) == 0 && len(req.Services) == 0 && len(req.Lines) == 0 {
		respondWithError(w, http.StatusBadRequest, "At least one service is required")
		return
	}

	for _, service := range req.Services {
		if service.Name == "" {
			respondWithError(w, http.StatusBadRequest, "Service name is required")
			return
		}
		if service.Price <= 0 {
			respondWithError(w, http.StatusBadRequest, "Service price must be positive")
			return
		}
	}

	if req.Status != "" && req.Status != "draft" && req.Status != "signed" && req.Status != "canceled" {
		respondWithError(w, http.StatusBadRequest, "Invalid act status")
		return
	}

	if _, err := time.Parse("02.01.2006", req.Date); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid act date format")
		return
	}

	// Проверяем уникальность номера акта в рамках договора
	contractID := req.ContractID
	if contractID == "" && req.CustomerID != "" && req.ContractNumber != "" {
		contract, err := h.db.GetContractByCustomerAndNumber(ctx, req.CustomerID, req.ContractNumber)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Contract not found")
			return
		}
		contractID = contract.ID
	}
	if contractID == "" {
		respondWithError(w, http.StatusBadRequest, "Contract ID is required")
		return
	}

	exists, err := h.db.CheckActNumberExists(ctx, contractID, req.Number, "")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to check act number")
		return
	}
	if exists {
		respondWithError(w, http.StatusConflict, "Act with this number already exists for this contract")
		return
	}

	act, err := h.db.CreateAct(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Act with this number already exists for this contract")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to create act")
		return
	}

	respondWithJSON(w, http.StatusCreated, models.ActResponse{Data: *act})
}

// LinkActInvoices обрабатывает POST /api/acts/{id}/invoices
func (h *Handlers) LinkActInvoices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actID := chi.URLParam(r, "id")
	if actID == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	var req models.LinkActInvoicesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.db.LinkActInvoices(ctx, actID, req.InvoiceIDs); err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to link act to invoices")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteAct обрабатывает DELETE /api/acts/{id}
func (h *Handlers) DeleteAct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	if err := h.db.DeleteAct(ctx, id); err != nil {
		if isForeignKeyViolation(err) {
			respondWithError(w, http.StatusConflict, "Не удалось удалить акт: он используется в других данных")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Act not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to delete act")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateAct обрабатывает PATCH /api/acts/{id}
func (h *Handlers) UpdateAct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	var req models.UpdateActRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	current, err := h.db.GetActByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Act not found")
		return
	}

	if current.Archived {
		if req.Archived == nil || *req.Archived {
			respondWithError(w, http.StatusBadRequest, "Archived act cannot be modified")
			return
		}
		if req.Number != nil || req.Date != nil || req.Status != nil {
			respondWithError(w, http.StatusBadRequest, "Archived act can only be unarchived")
			return
		}
	}

	if req.Number != nil && !isDigitsOnly(*req.Number) {
		respondWithError(w, http.StatusBadRequest, "Act number must be numeric")
		return
	}
	if req.Date != nil {
		if _, err := time.Parse("02.01.2006", *req.Date); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid act date format")
			return
		}
	}
	if req.Status != nil {
		if *req.Status != "draft" && *req.Status != "signed" && *req.Status != "canceled" {
			respondWithError(w, http.StatusBadRequest, "Invalid act status")
			return
		}
	}

	act, err := h.db.UpdateAct(ctx, id, req.Number, req.Date, req.Status, req.Archived)
	if err != nil {
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Act with this number already exists for this contract")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to update act")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ActResponse{Data: *act})
}

// AddActLine обрабатывает POST /api/acts/{id}/lines
func (h *Handlers) AddActLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID is required")
		return
	}

	var req models.AddActLineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	line := req.Line
	if line.ServiceID == "" && line.Title == "" {
		respondWithError(w, http.StatusBadRequest, "Service or title is required")
		return
	}
	if line.ServiceID == "" && line.Price <= 0 {
		respondWithError(w, http.StatusBadRequest, "Price must be positive")
		return
	}
	if line.Qty == 0 {
		line.Qty = 1
	}
	if line.Qty < 0 {
		respondWithError(w, http.StatusBadRequest, "Qty must be positive")
		return
	}

	if err := h.db.AddActLine(ctx, id, line); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Act not found")
			return
		}
		if err.Error() == "act is archived" {
			respondWithError(w, http.StatusBadRequest, "Archived act cannot be modified")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Failed to add act line")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateActLine обрабатывает PATCH /api/acts/{id}/lines/{lineID}
func (h *Handlers) UpdateActLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID and line ID are required")
		return
	}

	var req models.AddActLineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	line := normalizeLineInput(req.Line)
	if !validLineInput(line) {
		respondWithError(w, http.StatusBadRequest, "Invalid act line")
		return
	}

	if err := h.db.UpdateActLine(ctx, id, lineID, line); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Act line not found")
			return
		}
		if err.Error() == "act is archived" {
			respondWithError(w, http.StatusBadRequest, "Archived act cannot be modified")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Failed to update act line")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteActLine обрабатывает DELETE /api/acts/{id}/lines/{lineID}
func (h *Handlers) DeleteActLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		respondWithError(w, http.StatusBadRequest, "Act ID and line ID are required")
		return
	}

	if err := h.db.DeleteActLine(ctx, id, lineID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Act line not found")
			return
		}
		if err.Error() == "act is archived" {
			respondWithError(w, http.StatusBadRequest, "Archived act cannot be modified")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Failed to delete act line")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
