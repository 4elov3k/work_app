package handlers

import "invoices-backend/internal/models"

func normalizeLineInput(line models.InvoiceLineInput) models.InvoiceLineInput {
	if line.Qty == 0 {
		line.Qty = 1
	}
	if line.Unit == "" {
		line.Unit = "шт"
	}
	return line
}

func validLineInput(line models.InvoiceLineInput) bool {
	if line.ServiceID == "" && line.Title == "" {
		return false
	}
	if line.ServiceID == "" && line.Price <= 0 {
		return false
	}
	return line.Qty > 0
}
