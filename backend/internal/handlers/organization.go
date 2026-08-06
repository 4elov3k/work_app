package handlers

import (
	"net/http"

	"invoices-backend/internal/models"
)

// GetOrganization обрабатывает GET /api/organization — реквизиты собственной
// организации (продавца), используемые печатными формами и экспортом УПД.
func (h *Handlers) GetOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	org, err := h.db.GetActiveOrganization(ctx)
	if err != nil {
		respondNotFoundOrInternal(w, err, "Organization is not configured")
		return
	}

	respondWithJSON(w, http.StatusOK, models.OrganizationResponse{Data: *org})
}
