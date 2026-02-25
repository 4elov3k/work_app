package models

import "time"

// Service представляет услугу
type Service struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceResponse представляет ответ с одной услугой
type ServiceResponse struct {
	Data Service `json:"data"`
}

// ServiceListResponse представляет ответ со списком услуг
type ServiceListResponse struct {
	Data    []Service `json:"data"`
	Total   int       `json:"total"`
	Page    int       `json:"page"`
	PerPage int       `json:"per_page"`
}

// CreateServiceRequest представляет запрос на создание услуги
type CreateServiceRequest struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}
