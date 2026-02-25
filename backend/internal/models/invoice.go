package models

import "time"

// Invoice представляет счет
type Invoice struct {
	ID             string    `json:"id"`
	ContractID     string    `json:"contract_id"`
	CustomerID     string    `json:"customer_id"`
	Number         string    `json:"number"`
	Date           string    `json:"date"`
	Status         string    `json:"status"`
	TotalAmount    float64   `json:"total_amount"`
	Archived       bool      `json:"archived"`
	ContractNumber string    `json:"contract_number"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// InvoiceWithServices представляет счет с услугами
type InvoiceWithServices struct {
	Invoice
	Services []Service `json:"services"`
}

// InvoiceListResponse представляет ответ со списком счетов
type InvoiceListResponse struct {
	Data    []Invoice `json:"data"`
	Total   int       `json:"total"`
	Page    int       `json:"page"`
	PerPage int       `json:"per_page"`
}

// InvoiceResponse представляет ответ с одним счетом
type InvoiceResponse struct {
	Data Invoice `json:"data"`
}

// InvoiceWithServicesResponse представляет ответ со счетом и услугами
type InvoiceWithServicesResponse struct {
	Data InvoiceWithServices `json:"data"`
}

// CreateInvoiceRequest представляет запрос на создание счета
type CreateInvoiceRequest struct {
	ContractID     string                 `json:"contract_id"`
	CustomerID     string                 `json:"customer_id"`
	Number         string                 `json:"number"`
	Date           string                 `json:"date"`
	Status         string                 `json:"status"`
	ContractNumber string                 `json:"contract_number"`
	ServiceIDs     []string               `json:"service_ids"`
	Services       []CreateServiceRequest `json:"services"`
	Lines          []InvoiceLineInput     `json:"lines"`
}

// DuplicateInvoiceRequest представляет запрос на дублирование счета
type DuplicateInvoiceRequest struct {
	InvoiceID string `json:"invoice_id"`
	Number    string `json:"number"`
	Date      string `json:"date"`
}

// UpdateInvoiceRequest представляет запрос на обновление счета
type UpdateInvoiceRequest struct {
	Number   *string `json:"number"`
	Date     *string `json:"date"`
	Status   *string `json:"status"`
	Archived *bool   `json:"archived"`
}

// InvoiceLineInput представляет входную строку счета
type InvoiceLineInput struct {
	ServiceID string  `json:"service_id"`
	Title     string  `json:"title"`
	Unit      string  `json:"unit"`
	VAT       float64 `json:"vat"`
	Price     float64 `json:"price"`
	Qty       float64 `json:"qty"`
}

// AddInvoiceLineRequest представляет запрос на добавление строки счета
type AddInvoiceLineRequest struct {
	Line InvoiceLineInput `json:"line"`
}
