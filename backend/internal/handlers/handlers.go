package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/lib/pq"

	"invoices-backend/internal/database"
	"invoices-backend/internal/docparse"
	"invoices-backend/internal/models"
	"invoices-backend/internal/redmine"
	"invoices-backend/internal/saby"
	"invoices-backend/internal/sheetsync"
)

// Handlers содержит все HTTP обработчики
type Handlers struct {
	db        *database.DB
	redmine   *redmine.Client
	saby      *saby.Client
	docparse  *docparse.Client
	sheetsync *sheetsync.Client
}

// NewHandlers создает новый экземпляр handlers
func NewHandlers(db *database.DB) *Handlers {
	return &Handlers{
		db:        db,
		redmine:   redmine.NewFromEnv(),
		saby:      saby.NewFromEnv(),
		docparse:  docparse.NewFromEnv(),
		sheetsync: sheetsync.NewFromEnv(),
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
// Checked via errors.Is against database.ErrNotFound (wrapped into every
// "<entity> not found" error the database package returns for sql.ErrNoRows)
// rather than matching error message text, so rewording one of those
// messages can't silently break this classification.
func isRecordNotFoundError(err error) bool {
	return errors.Is(err, database.ErrNotFound)
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

// respondLinkedRecordOrInternal answers a failure to load a record that a
// parent document merely *references* (e.g. an act's customer or contract)
// as 400 — a missing reference is a data-integrity problem with the parent,
// not a 404 on the URL's own resource — and as 500 (logging the real cause)
// for anything else. Shared by acts.go/invoices.go's export handlers, which
// both do this same customer-then-contract lookup.
func respondLinkedRecordOrInternal(w http.ResponseWriter, err error, notFoundMessage, logContext string) {
	if isRecordNotFoundError(err) {
		respondWithError(w, http.StatusBadRequest, notFoundMessage)
		return
	}
	log.Printf("%s: %v", logContext, err)
	respondWithError(w, http.StatusInternalServerError, "Internal server error")
}
