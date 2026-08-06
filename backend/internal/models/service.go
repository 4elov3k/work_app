package models

import "time"

// Service представляет услугу — как разовую (созданную вручную), так и
// позицию из каталога (Section непустой), например, из стандартного прайса.
type Service struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Unit         string    `json:"unit"`
	VAT          float64   `json:"vat"`
	Price        float64   `json:"price"`
	Qty          float64   `json:"qty"`
	Amount       float64   `json:"amount"`
	Section      string    `json:"section,omitempty"`
	PricePerHour float64   `json:"price_per_hour,omitempty"`
	HoursPerUnit float64   `json:"hours_per_unit,omitempty"`
	Archived     bool      `json:"archived"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	Unit         string  `json:"unit,omitempty"`
	Section      string  `json:"section,omitempty"`
	PricePerHour float64 `json:"price_per_hour,omitempty"`
	HoursPerUnit float64 `json:"hours_per_unit,omitempty"`
}

// ServiceCatalogSection представляет один раздел каталога услуг со списком позиций.
type ServiceCatalogSection struct {
	Section string    `json:"section"`
	Items   []Service `json:"items"`
}

// ServiceCatalogResponse представляет ответ с каталогом услуг, сгруппированным по разделам.
type ServiceCatalogResponse struct {
	Data []ServiceCatalogSection `json:"data"`
}
