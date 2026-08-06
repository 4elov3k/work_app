package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/export/updxml"
	"invoices-backend/internal/models"
)

// GetInvoices обрабатывает GET /api/invoices
func (h *Handlers) GetInvoices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Параметры запроса
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

	// Получаем данные из БД
	invoices, total, err := h.db.GetInvoices(ctx, customerID, contractID, archived, page, perPage)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get invoices")
		return
	}

	// Формируем ответ
	response := models.InvoiceListResponse{
		Data:    invoices,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GetInvoiceByID обрабатывает GET /api/invoices/{id}
func (h *Handlers) GetInvoiceByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	invoice, err := h.db.GetInvoiceByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Invoice not found")
		return
	}

	response := models.InvoiceResponse{
		Data: *invoice,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GetInvoiceWithServices обрабатывает GET /api/invoices/{id}/services
func (h *Handlers) GetInvoiceWithServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	invoice, err := h.db.GetInvoiceWithServices(ctx, id)
	if err != nil {
		// Логируем реальную ошибку для отладки
		log.Printf("Error getting invoice with services (ID: %s): %v", id, err)
		respondNotFoundOrInternal(w, err, "Invoice not found")
		return
	}

	response := models.InvoiceWithServicesResponse{
		Data: *invoice,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// ExportInvoiceXML обрабатывает GET /api/invoices/{id}/export/upd-xml
func (h *Handlers) ExportInvoiceXML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	invoice, err := h.db.GetInvoiceWithServices(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Invoice not found")
		return
	}

	customer, err := h.db.GetCustomerByID(ctx, invoice.CustomerID)
	if err != nil {
		if isRecordNotFoundError(err) {
			respondWithError(w, http.StatusBadRequest, "Customer not found for invoice")
			return
		}
		log.Printf("Error loading customer for invoice export (invoice ID: %s): %v", id, err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	contract, err := h.db.GetContractByID(ctx, invoice.ContractID)
	if err != nil {
		if isRecordNotFoundError(err) {
			respondWithError(w, http.StatusBadRequest, "Contract not found for invoice")
			return
		}
		log.Printf("Error loading contract for invoice export (invoice ID: %s): %v", id, err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	org, err := h.db.GetActiveOrganization(ctx)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Organization is not configured")
		return
	}

	data, filename, err := updxml.BuildInvoiceXML(*invoice, *customer, *contract, updxml.SellerFromOrganization(*org))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="invoice.xml"; filename*=UTF-8''`+url.PathEscape(filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// CreateInvoice обрабатывает POST /api/invoices
func (h *Handlers) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if req.Number == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice number is required")
		return
	}
	if !isDigitsOnly(req.Number) {
		respondWithError(w, http.StatusBadRequest, "Invoice number must be numeric")
		return
	}

	if req.Date == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice date is required")
		return
	}

	if req.Status != "" && req.Status != "draft" && req.Status != "issued" && req.Status != "paid" && req.Status != "canceled" {
		respondWithError(w, http.StatusBadRequest, "Invalid invoice status")
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

	if _, err := time.Parse("02.01.2006", req.Date); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid invoice date format")
		return
	}

	// Проверяем уникальность номера счета
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

	exists, err := h.db.CheckInvoiceNumberExists(ctx, contractID, req.Number, "")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to check invoice number")
		return
	}
	if exists {
		respondWithError(w, http.StatusConflict, "Invoice with this number already exists for this contract")
		return
	}

	// Создаем счет
	invoice, err := h.db.CreateInvoice(ctx, req)
	if err != nil {
		log.Printf("CreateInvoice failed: %+v; payload={contract_id:%q customer_id:%q contract_number:%q number:%q date:%q service_ids:%d services:%d lines:%d}",
			err, req.ContractID, req.CustomerID, req.ContractNumber, req.Number, req.Date, len(req.ServiceIDs), len(req.Services), len(req.Lines))
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Invoice with this number already exists for this contract")
			return
		}
		message := err.Error()
		switch {
		case strings.Contains(message, "contract not found"):
			respondWithError(w, http.StatusBadRequest, "Contract not found")
			return
		case strings.Contains(message, "contract_id is required"):
			respondWithError(w, http.StatusBadRequest, "Contract ID is required")
			return
		case strings.Contains(message, "at least one line is required"):
			respondWithError(w, http.StatusBadRequest, "At least one service is required")
			return
		case strings.Contains(message, "one or more services not found"), strings.Contains(message, "service not found"):
			respondWithError(w, http.StatusBadRequest, "One or more services not found")
			return
		case strings.Contains(message, "invalid service"), strings.Contains(message, "invalid invoice line"):
			respondWithError(w, http.StatusBadRequest, "Invalid invoice data")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to create invoice")
		return
	}

	response := models.InvoiceResponse{
		Data: *invoice,
	}

	respondWithJSON(w, http.StatusCreated, response)
}

// DuplicateInvoice обрабатывает POST /api/invoices/duplicate
func (h *Handlers) DuplicateInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.DuplicateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if req.InvoiceID == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	if req.Number == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice number is required")
		return
	}

	if req.Date == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice date is required")
		return
	}

	// Дублируем счет
	invoice, err := h.db.DuplicateInvoice(ctx, req)
	if err != nil {
		log.Printf("Failed to duplicate invoice: %v", err)
		if err.Error() == "invoice number already exists for this customer" || isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Invoice with this number already exists for this contract")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to duplicate invoice")
		return
	}

	response := models.InvoiceResponse{
		Data: *invoice,
	}

	respondWithJSON(w, http.StatusCreated, response)
}

// DeleteInvoice обрабатывает DELETE /api/invoices/{id}
func (h *Handlers) DeleteInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	if err := h.db.DeleteInvoice(ctx, id); err != nil {
		if isForeignKeyViolation(err) {
			respondWithError(w, http.StatusConflict, "Не удалось удалить счёт: он используется в других данных")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Invoice not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to delete invoice")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateInvoice обрабатывает PATCH /api/invoices/{id}
func (h *Handlers) UpdateInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	var req models.UpdateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	current, err := h.db.GetInvoiceByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Invoice not found")
		return
	}

	if current.Archived {
		// allow only unarchive
		if req.Archived == nil || *req.Archived {
			respondWithError(w, http.StatusBadRequest, "Archived invoice cannot be modified")
			return
		}
		if req.Number != nil || req.Date != nil || req.Status != nil {
			respondWithError(w, http.StatusBadRequest, "Archived invoice can only be unarchived")
			return
		}
	}

	if req.Number != nil && !isDigitsOnly(*req.Number) {
		respondWithError(w, http.StatusBadRequest, "Invoice number must be numeric")
		return
	}

	if req.Date != nil {
		if _, err := time.Parse("02.01.2006", *req.Date); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid invoice date format")
			return
		}
	}

	if req.Status != nil {
		if *req.Status != "draft" && *req.Status != "issued" && *req.Status != "paid" && *req.Status != "canceled" {
			respondWithError(w, http.StatusBadRequest, "Invalid invoice status")
			return
		}
	}

	invoice, err := h.db.UpdateInvoice(ctx, id, req.Number, req.Date, req.Status, req.Archived)
	if err != nil {
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Invoice with this number already exists for this contract")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to update invoice")
		return
	}

	respondWithJSON(w, http.StatusOK, models.InvoiceResponse{Data: *invoice})
}

// AddInvoiceLine обрабатывает POST /api/invoices/{id}/lines
func (h *Handlers) AddInvoiceLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	var req models.AddInvoiceLineRequest
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

	if err := h.db.AddInvoiceLine(ctx, id, line); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Invoice not found")
			return
		}
		if err.Error() == "invoice is archived" {
			respondWithError(w, http.StatusBadRequest, "Archived invoice cannot be modified")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Failed to add invoice line")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateInvoiceLine обрабатывает PATCH /api/invoices/{id}/lines/{lineID}
func (h *Handlers) UpdateInvoiceLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID and line ID are required")
		return
	}

	var req models.AddInvoiceLineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	line := normalizeLineInput(req.Line)
	if !validLineInput(line) {
		respondWithError(w, http.StatusBadRequest, "Invalid invoice line")
		return
	}

	if err := h.db.UpdateInvoiceLine(ctx, id, lineID, line); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Invoice line not found")
			return
		}
		if err.Error() == "invoice is archived" {
			respondWithError(w, http.StatusBadRequest, "Archived invoice cannot be modified")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Failed to update invoice line")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteInvoiceLine обрабатывает DELETE /api/invoices/{id}/lines/{lineID}
func (h *Handlers) DeleteInvoiceLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID and line ID are required")
		return
	}

	if err := h.db.DeleteInvoiceLine(ctx, id, lineID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Invoice line not found")
			return
		}
		if err.Error() == "invoice is archived" {
			respondWithError(w, http.StatusBadRequest, "Archived invoice cannot be modified")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Failed to delete invoice line")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateActFromInvoice обрабатывает POST /api/invoices/{id}/act
func (h *Handlers) CreateActFromInvoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	var req models.CreateActFromInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Number == "" || req.Date == "" {
		respondWithError(w, http.StatusBadRequest, "Act number and date are required")
		return
	}
	if !isDigitsOnly(req.Number) {
		respondWithError(w, http.StatusBadRequest, "Act number must be numeric")
		return
	}
	if req.Status != "" && req.Status != "draft" && req.Status != "signed" && req.Status != "canceled" {
		respondWithError(w, http.StatusBadRequest, "Invalid act status")
		return
	}
	if _, err := time.Parse("02.01.2006", req.Date); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid act date format")
		return
	}

	invoice, err := h.db.GetInvoiceByID(ctx, id)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Invoice not found")
		return
	}
	if invoice.Archived {
		respondWithError(w, http.StatusBadRequest, "Archived invoice cannot be modified")
		return
	}

	exists, err := h.db.CheckActNumberExists(ctx, invoice.ContractID, req.Number, "")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to check act number")
		return
	}
	if exists {
		respondWithError(w, http.StatusConflict, "Act with this number already exists for this contract")
		return
	}

	act, err := h.db.CreateActFromInvoice(ctx, id, req.Number, req.Date, req.Status)
	if err != nil {
		if err.Error() == "invoice not found" {
			respondWithError(w, http.StatusNotFound, "Invoice not found")
			return
		}
		if isUniqueViolation(err) {
			respondWithError(w, http.StatusConflict, "Act with this number already exists for this contract")
			return
		}
		log.Printf("CreateActFromInvoice failed: %v", err)
		respondWithError(w, http.StatusBadRequest, "Failed to create act from invoice")
		return
	}

	respondWithJSON(w, http.StatusCreated, models.ActResponse{Data: *act})
}
