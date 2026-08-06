package models

import "time"

// Act представляет акт выполненных работ
type Act struct {
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

// ActWithServices представляет акт с услугами
type ActWithServices struct {
	Act
	Services []Service `json:"services"`
	Invoices []Invoice `json:"invoices"`
}

// ActListResponse представляет ответ со списком актов
type ActListResponse struct {
	Data    []Act `json:"data"`
	Total   int   `json:"total"`
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
}

// ActResponse представляет ответ с одним актом
type ActResponse struct {
	Data Act `json:"data"`
}

// ActWithServicesResponse представляет ответ с актом и услугами
type ActWithServicesResponse struct {
	Data ActWithServices `json:"data"`
}

// CreateActRequest представляет запрос на создание акта
type CreateActRequest struct {
	ContractID     string                 `json:"contract_id"`
	CustomerID     string                 `json:"customer_id"`
	Number         string                 `json:"number"`
	Date           string                 `json:"date"`
	Status         string                 `json:"status"`
	ContractNumber string                 `json:"contract_number"`
	ServiceIDs     []string               `json:"service_ids"`
	Services       []CreateServiceRequest `json:"services"`
	Lines          []InvoiceLineInput     `json:"lines"`
	InvoiceIDs     []string               `json:"invoice_ids"`
}

// CreateActFromInvoiceRequest представляет запрос на создание акта из счета
type CreateActFromInvoiceRequest struct {
	Number string `json:"number"`
	Date   string `json:"date"`
	Status string `json:"status"`
}

// LinkActInvoicesRequest представляет запрос на привязку актов к счетам
type LinkActInvoicesRequest struct {
	InvoiceIDs []string `json:"invoice_ids"`
}

// AddActLineRequest представляет запрос на добавление строки акта
type AddActLineRequest struct {
	Line InvoiceLineInput `json:"line"`
}

// UpdateActRequest представляет запрос на обновление акта
type UpdateActRequest struct {
	Number   *string `json:"number"`
	Date     *string `json:"date"`
	Status   *string `json:"status"`
	Archived *bool   `json:"archived"`
}
