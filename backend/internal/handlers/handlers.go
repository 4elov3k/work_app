package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/lib/pq"

	"invoices-backend/internal/callreport"
	"invoices-backend/internal/database"
	"invoices-backend/internal/docparse"
	"invoices-backend/internal/models"
	"invoices-backend/internal/pbx"
	"invoices-backend/internal/redmine"
	"invoices-backend/internal/saby"
	"invoices-backend/internal/sheetsync"
	"invoices-backend/internal/transcribe"
	"invoices-backend/internal/zvonari"
)

// Handlers содержит все HTTP обработчики
type Handlers struct {
	db        *database.DB
	redmine   *redmine.Client
	saby      *saby.Client
	docparse  *docparse.Client
	sheetsync *sheetsync.Client
	zvonari   *zvonari.Service
}

// NewHandlers создает новый экземпляр handlers
func NewHandlers(db *database.DB) *Handlers {
	zvonariService := zvonari.NewService(db, pbx.NewFromEnv(), transcribe.NewFromEnv(), callreport.NewFromEnv())
	return &Handlers{
		db:        db,
		redmine:   redmine.NewFromEnv(),
		saby:      saby.NewFromEnv(),
		docparse:  docparse.NewFromEnv(),
		sheetsync: sheetsync.NewFromEnv(),
		zvonari:   zvonariService,
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

// isRecordNotFoundError reports whether err is the database layer's "X not found"
// sentinel for a missing row, as opposed to a real database/connection failure.
// The database package returns a bare "<entity> not found" message for
// sql.ErrNoRows and wraps everything else (e.g. "failed to get invoice: ...."),
// so a "not found" suffix reliably distinguishes the two without changing the
// exact error text other code already matches against.
func isRecordNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasSuffix(err.Error(), "not found")
}

// respondNotFoundOrInternal answers a "get by ID" failure as 404 when the
// database layer reports a missing row, and as 500 (logging the real cause)
// for anything else — so a database outage is never mistaken for a deleted record.
func respondNotFoundOrInternal(w http.ResponseWriter, err error, notFoundMessage string) {
	if isRecordNotFoundError(err) {
		respondWithError(w, http.StatusNotFound, notFoundMessage)
		return
	}
	log.Printf("internal error: %v", err)
	respondWithError(w, http.StatusInternalServerError, "Internal server error")
}
