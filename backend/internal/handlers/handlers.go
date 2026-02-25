package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lib/pq"

	"invoices-backend/internal/database"
	"invoices-backend/internal/models"
)

// Handlers содержит все HTTP обработчики
type Handlers struct {
	db *database.DB
}

// NewHandlers создает новый экземпляр handlers
func NewHandlers(db *database.DB) *Handlers {
	return &Handlers{db: db}
}

// respondWithJSON отправляет JSON ответ
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error", "code": 500}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// respondWithError отправляет ошибку в формате JSON
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23503"
	}
	return false
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}
	return false
}
