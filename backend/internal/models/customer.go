package models

import "time"

// Customer представляет контрагента (клиента)
type Customer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Fullname  string    `json:"fullname"`
	Address   string    `json:"address"`
	INN       string    `json:"inn"`
	KPP       string    `json:"kpp"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CustomerListResponse представляет ответ со списком контрагентов
type CustomerListResponse struct {
	Data    []Customer `json:"data"`
	Total   int        `json:"total"`
	Page    int        `json:"page"`
	PerPage int        `json:"per_page"`
}

// CustomerResponse представляет ответ с одним контрагентом
type CustomerResponse struct {
	Data Customer `json:"data"`
}

// CustomerLookup представляет найденные по ИНН реквизиты контрагента.
type CustomerLookup struct {
	Name     string `json:"name"`
	Fullname string `json:"fullname"`
	Address  string `json:"address"`
	INN      string `json:"inn"`
	KPP      string `json:"kpp"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

// CustomerLookupResponse представляет ответ проверки контрагента по ИНН.
type CustomerLookupResponse struct {
	Data CustomerLookup `json:"data"`
}

// CreateCustomerRequest представляет запрос на создание контрагента
type CreateCustomerRequest struct {
	Name     string `json:"name"`
	Fullname string `json:"fullname"`
	Address  string `json:"address"`
	INN      string `json:"inn"`
	KPP      string `json:"kpp"`
}
