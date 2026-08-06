package models

import "time"

// ContractAppendix представляет приложение к договору (смету) — печатаемый
// документ со строками работ, сгруппированными по разделам.
type ContractAppendix struct {
	ID          string    `json:"id"`
	ContractID  string    `json:"contract_id"`
	Number      string    `json:"number"`
	Date        string    `json:"date"`
	Status      string    `json:"status"`
	TotalAmount float64   `json:"total_amount"`
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ContractAppendixLine представляет строку приложения — либо позицию из
// каталога услуг (ServiceID непустой), либо кастомную строку.
type ContractAppendixLine struct {
	ID         string  `json:"id"`
	AppendixID string  `json:"appendix_id"`
	ServiceID  string  `json:"service_id,omitempty"`
	Section    string  `json:"section"`
	Position   int     `json:"position"`
	Title      string  `json:"title"`
	Unit       string  `json:"unit"`
	Price      float64 `json:"price"`
	Qty        float64 `json:"qty"`
	Amount     float64 `json:"amount"`
}

// ContractAppendixWithLines представляет приложение вместе со строками.
type ContractAppendixWithLines struct {
	ContractAppendix
	Lines []ContractAppendixLine `json:"lines"`
}

// ContractAppendixLineInput представляет входную строку приложения при создании/обновлении.
type ContractAppendixLineInput struct {
	ServiceID string  `json:"service_id,omitempty"`
	Section   string  `json:"section"`
	Title     string  `json:"title"`
	Unit      string  `json:"unit"`
	Price     float64 `json:"price"`
	Qty       float64 `json:"qty"`
}

// CreateContractAppendixRequest представляет запрос на создание приложения.
type CreateContractAppendixRequest struct {
	ContractID string                      `json:"contract_id"`
	Number     string                      `json:"number"`
	Date       string                      `json:"date"`
	Status     string                      `json:"status"`
	Lines      []ContractAppendixLineInput `json:"lines"`
}

// UpdateContractAppendixRequest представляет запрос на обновление приложения.
type UpdateContractAppendixRequest struct {
	Number   *string `json:"number"`
	Date     *string `json:"date"`
	Status   *string `json:"status"`
	Archived *bool   `json:"archived"`
}

// AddContractAppendixLineRequest представляет запрос на добавление строки приложения.
type AddContractAppendixLineRequest struct {
	Line ContractAppendixLineInput `json:"line"`
}

// ContractAppendixResponse представляет ответ с одним приложением.
type ContractAppendixResponse struct {
	Data ContractAppendix `json:"data"`
}

// ContractAppendixWithLinesResponse представляет ответ с приложением и строками.
type ContractAppendixWithLinesResponse struct {
	Data ContractAppendixWithLines `json:"data"`
}

// ContractAppendixListResponse представляет ответ со списком приложений.
type ContractAppendixListResponse struct {
	Data    []ContractAppendix `json:"data"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	PerPage int                `json:"per_page"`
}
