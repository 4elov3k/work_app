package models

import "time"

// Caller представляет звонаря — внутренний номер (extension) в OnlinePBX
type Caller struct {
	ID           string    `json:"id"`
	PBXExtension string    `json:"pbx_extension"`
	Name         string    `json:"name"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CallerListResponse представляет ответ со списком звонарей
type CallerListResponse struct {
	Data []Caller `json:"data"`
}
