package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/lib/pq"

	"invoices-backend/internal/database"
	"invoices-backend/internal/models"
	"invoices-backend/internal/redmine"
	"invoices-backend/internal/saby"
)

// Handlers содержит все HTTP обработчики
type Handlers struct {
	db      *database.DB
	redmine *redmine.Client
	saby    *saby.Client
}

// NewHandlers создает новый экземпляр handlers
func NewHandlers(db *database.DB) *Handlers {
	return &Handlers{
		db:      db,
		redmine: redmine.NewFromEnv(),
		saby:    saby.NewFromEnv(),
	}
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

func (h *Handlers) RequireAgentToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := os.Getenv("AGENT_API_TOKEN")
		if expected == "" {
			respondWithError(w, http.StatusServiceUnavailable, "Agent API token is not configured")
			return
		}

		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token != expected {
			respondWithError(w, http.StatusUnauthorized, "Invalid agent API token")
			return
		}

		next.ServeHTTP(w, r)
	})
}
