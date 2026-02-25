package models

import "time"

// Contract представляет договор
type Contract struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Number     string    `json:"number"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	Topic      string    `json:"topic"`
	StartDate  string    `json:"start_date"`
	EndDate    string    `json:"end_date"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ContractListResponse представляет ответ со списком договоров
type ContractListResponse struct {
	Data    []Contract `json:"data"`
	Total   int        `json:"total"`
	Page    int        `json:"page"`
	PerPage int        `json:"per_page"`
}

// ContractResponse представляет ответ с одним договором
type ContractResponse struct {
	Data Contract `json:"data"`
}

// CreateContractRequest представляет запрос на создание договора
type CreateContractRequest struct {
	CustomerID string `json:"customer_id"`
	Number     string `json:"number"`
	Currency   string `json:"currency"`
	Status     string `json:"status"`
	Topic      string `json:"topic"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}
