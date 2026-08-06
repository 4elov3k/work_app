package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
)

// GetServiceCatalog обрабатывает GET /api/services/catalog — позиции
// стандартного прайса, сгруппированные по разделам, для выбора при
// формировании приложения к договору.
func (h *Handlers) GetServiceCatalog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sections, err := h.db.GetServiceCatalog(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get service catalog")
		return
	}
	respondWithJSON(w, http.StatusOK, models.ServiceCatalogResponse{Data: sections})
}

// GetServices обрабатывает GET /api/services
func (h *Handlers) GetServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	services, total, err := h.db.GetServices(ctx, page, perPage)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get services")
		return
	}

	respondWithJSON(w, http.StatusOK, models.ServiceListResponse{
		Data:    services,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// CreateService обрабатывает POST /api/services
func (h *Handlers) CreateService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Service name is required")
		return
	}

	if req.Price <= 0 {
		respondWithError(w, http.StatusBadRequest, "Service price must be positive")
		return
	}

	// Создаем услугу
	service, err := h.db.CreateService(ctx, req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create service")
		return
	}

	response := models.ServiceResponse{
		Data: *service,
	}

	respondWithJSON(w, http.StatusCreated, response)
}

// DeleteService обрабатывает DELETE /api/services/{id}
func (h *Handlers) DeleteService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Service ID is required")
		return
	}

	if err := h.db.DeleteService(ctx, id); err != nil {
		if isForeignKeyViolation(err) {
			respondWithError(w, http.StatusConflict, "Услуга используется в приложениях к договору и не может быть удалена")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Service not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to delete service")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
