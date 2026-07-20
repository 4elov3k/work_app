package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
)

// GetCustomers обрабатывает GET /api/customers
func (h *Handlers) GetCustomers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Параметры запроса
	search := r.URL.Query().Get("search")
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

	// Получаем данные из БД
	customers, total, err := h.db.GetCustomers(ctx, search, page, perPage)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get customers")
		return
	}

	// Формируем ответ
	response := models.CustomerListResponse{
		Data:    customers,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GetCustomerByID обрабатывает GET /api/customers/{id}
func (h *Handlers) GetCustomerByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	customer, err := h.db.GetCustomerByID(ctx, id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Customer not found")
		return
	}

	response := models.CustomerResponse{
		Data: *customer,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// CreateCustomer обрабатывает POST /api/customers
func (h *Handlers) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Customer name is required")
		return
	}

	if req.Fullname == "" {
		respondWithError(w, http.StatusBadRequest, "Customer fullname is required")
		return
	}

	if req.INN == "" {
		respondWithError(w, http.StatusBadRequest, "Customer INN is required")
		return
	}

	if req.Address == "" {
		respondWithError(w, http.StatusBadRequest, "Customer address is required")
		return
	}

	req.INN = digitsOnly(req.INN)
	req.KPP = digitsOnly(req.KPP)
	if len(req.INN) != 10 && len(req.INN) != 12 {
		respondWithError(w, http.StatusBadRequest, "Customer INN must be 10 or 12 digits")
		return
	}

	if len(req.INN) == 10 && len(req.KPP) != 9 {
		respondWithError(w, http.StatusBadRequest, "Customer KPP must be 9 digits for organizations")
		return
	}

	// Создаем контрагента
	customer, err := h.db.CreateCustomer(ctx, req)
	if err != nil {
		log.Printf("CreateCustomer failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create customer")
		return
	}

	response := models.CustomerResponse{
		Data: *customer,
	}

	respondWithJSON(w, http.StatusCreated, response)
}
