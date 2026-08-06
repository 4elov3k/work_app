package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"invoices-backend/internal/models"
)

const dadataPartyLookupURL = "https://suggestions.dadata.ru/suggestions/api/4_1/rs/findById/party"

type dadataPartyRequest struct {
	Query      string `json:"query"`
	Count      int    `json:"count"`
	KPP        string `json:"kpp,omitempty"`
	BranchType string `json:"branch_type,omitempty"`
	Type       string `json:"type,omitempty"`
}

type dadataPartyResponse struct {
	Suggestions []dadataPartySuggestion `json:"suggestions"`
}

type dadataPartySuggestion struct {
	Value string `json:"value"`
	Data  struct {
		KPP     string `json:"kpp"`
		Type    string `json:"type"`
		INN     string `json:"inn"`
		Address struct {
			Value             string `json:"value"`
			UnrestrictedValue string `json:"unrestricted_value"`
		} `json:"address"`
		Name struct {
			ShortWithOPF string `json:"short_with_opf"`
			FullWithOPF  string `json:"full_with_opf"`
		} `json:"name"`
		State struct {
			Status string `json:"status"`
		} `json:"state"`
		Management struct {
			Name string `json:"name"`
			Post string `json:"post"`
		} `json:"management"`
	} `json:"data"`
}

// LookupCustomerByINN обрабатывает GET /api/customers/lookup?inn=...
func (h *Handlers) LookupCustomerByINN(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(os.Getenv("DADATA_API_KEY"))
	if token == "" {
		respondWithError(w, http.StatusServiceUnavailable, "DADATA_API_KEY is not configured")
		return
	}

	inn := digitsOnly(r.URL.Query().Get("inn"))
	kpp := digitsOnly(r.URL.Query().Get("kpp"))
	if len(inn) != 10 && len(inn) != 12 {
		respondWithError(w, http.StatusBadRequest, "INN must contain 10 or 12 digits")
		return
	}
	if kpp != "" && len(kpp) != 9 {
		respondWithError(w, http.StatusBadRequest, "KPP must contain 9 digits")
		return
	}

	payload := dadataPartyRequest{
		Query: inn,
		Count: 1,
	}
	if len(inn) == 10 {
		payload.Type = "LEGAL"
		if kpp != "" {
			payload.KPP = kpp
		} else {
			payload.BranchType = "MAIN"
		}
	} else {
		payload.Type = "INDIVIDUAL"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to build lookup request")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, dadataPartyLookupURL, bytes.NewReader(body))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to build lookup request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+token)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		respondWithError(w, http.StatusBadGateway, fmt.Sprintf("Failed to lookup INN: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respondWithError(w, http.StatusBadGateway, fmt.Sprintf("DaData lookup failed with HTTP %d", resp.StatusCode))
		return
	}

	var result dadataPartyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondWithError(w, http.StatusBadGateway, "Failed to parse lookup response")
		return
	}
	if len(result.Suggestions) == 0 {
		respondWithError(w, http.StatusNotFound, "Customer not found by INN")
		return
	}

	found := result.Suggestions[0]
	address := found.Data.Address.UnrestrictedValue
	if strings.TrimSpace(address) == "" {
		address = found.Data.Address.Value
	}
	fullname := found.Data.Name.FullWithOPF
	if strings.TrimSpace(fullname) == "" {
		fullname = found.Value
	}
	name := found.Data.Name.ShortWithOPF
	if strings.TrimSpace(name) == "" {
		name = found.Value
	}

	respondWithJSON(w, http.StatusOK, models.CustomerLookupResponse{
		Data: models.CustomerLookup{
			Name:            name,
			Fullname:        fullname,
			Address:         address,
			INN:             found.Data.INN,
			KPP:             found.Data.KPP,
			Type:            found.Data.Type,
			Status:          found.Data.State.Status,
			ContactPerson:   found.Data.Management.Name,
			ContactPosition: found.Data.Management.Post,
		},
	})
}
