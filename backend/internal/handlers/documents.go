package handlers

import (
	"io"
	"log"
	"net/http"

	"invoices-backend/internal/docparse"
)

const maxDocumentUploadBytes = 20 << 20 // 20 MB

// ParseContractDocument accepts an uploaded scan (PDF or image), extracts its
// text via the Hermes ocr-extract service, and returns the handful of fields
// the "create contract" form can prefill. It never invents a value — fields
// the regex extraction didn't find come back empty, and the user fills them
// in by hand, same as the manual-form path.
func (h *Handlers) ParseContractDocument(w http.ResponseWriter, r *http.Request) {
	if !h.docparse.Configured() {
		respondWithError(w, http.StatusServiceUnavailable, "Распознавание документов недоступно (не настроен OCR_SERVICE_URL)")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxDocumentUploadBytes)
	if err := r.ParseMultipartForm(maxDocumentUploadBytes); err != nil {
		respondWithError(w, http.StatusBadRequest, "Файл слишком большой или повреждён")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Файл не передан")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Не удалось прочитать файл")
		return
	}

	result, err := h.docparse.Extract(r.Context(), header.Filename, data)
	if err != nil {
		log.Printf("docparse: extract failed for %q: %v", header.Filename, err)
		respondWithError(w, http.StatusBadGateway, "Не удалось распознать документ")
		return
	}

	var sellerINN string
	if org, err := h.db.GetActiveOrganization(r.Context()); err == nil && org != nil {
		sellerINN = org.INN
	}

	fields := docparse.ExtractContractFields(result.Text, sellerINN)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"fields":   fields,
			"pages":    len(result.Pages),
			"doc_type": result.Type,
		},
	})
}
