package accounting

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"invoices-backend/internal/contracttopics"
	"invoices-backend/internal/database"
	"invoices-backend/internal/export/updxml"
	"invoices-backend/internal/models"
)

const defaultUserID = "hermes"

var tokenRe = regexp.MustCompile(`^[a-f0-9]{64}$`)

type contextKey string

const userContextKey contextKey = "accounting_user"

func WithUser(ctx context.Context, userID string) context.Context {
	if strings.TrimSpace(userID) == "" {
		userID = defaultUserID
	}
	return context.WithValue(ctx, userContextKey, userID)
}

func UserFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(userContextKey).(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return defaultUserID
}

type Config struct {
	DocumentStoragePath string
	TokenTTL            time.Duration
	AllowFinalization   bool
	AllowSending        bool
}

type Service struct {
	db     *database.DB
	config Config
}

func NewService(db *database.DB, config Config) *Service {
	if config.DocumentStoragePath == "" {
		config.DocumentStoragePath = "/data/documents"
	}
	if config.TokenTTL == 0 {
		config.TokenTTL = 15 * time.Minute
	}
	return &Service{db: db, config: config}
}

type AccountingError struct {
	Code            string         `json:"code"`
	Message         string         `json:"message"`
	Details         map[string]any `json:"details,omitempty"`
	Recoverable     bool           `json:"recoverable"`
	SuggestedAction string         `json:"suggested_action,omitempty"`
	UnderlyingError string         `json:"-"`
}

func (e *AccountingError) Error() string {
	return e.Message
}

func appError(code, message string, recoverable bool, suggested string) *AccountingError {
	return &AccountingError{Code: code, Message: message, Recoverable: recoverable, SuggestedAction: suggested}
}

type Organization struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	FullName          string         `json:"full_name"`
	ShortName         string         `json:"short_name"`
	INN               string         `json:"inn"`
	KPP               string         `json:"kpp,omitempty"`
	OGRN              string         `json:"ogrn,omitempty"`
	LegalAddress      string         `json:"legal_address,omitempty"`
	PostalAddress     string         `json:"postal_address,omitempty"`
	Phone             string         `json:"phone,omitempty"`
	BankAccount       string         `json:"bank_account,omitempty"`
	BankName          string         `json:"bank_name,omitempty"`
	BankBIK           string         `json:"bank_bik,omitempty"`
	BankCorrAccount   string         `json:"bank_corr_account,omitempty"`
	TaxRegime         string         `json:"tax_regime"`
	VATMode           string         `json:"vat_mode"`
	Timezone          string         `json:"timezone"`
	EDOParticipantID  string         `json:"edo_participant_id,omitempty"`
	Signer            map[string]any `json:"signer,omitempty"`
	NumberingSettings map[string]any `json:"numbering_settings,omitempty"`
	Active            bool           `json:"active"`
}

// sellerFromOrganization adapts the fetched Organization (whose Signer is a
// raw JSONB map) into the updxml.Seller shape used for document generation.
func sellerFromOrganization(org *Organization) updxml.Seller {
	signerString := func(key string) string {
		if org.Signer == nil {
			return ""
		}
		if value, ok := org.Signer[key].(string); ok {
			return value
		}
		return ""
	}
	return updxml.Seller{
		FullName:        org.FullName,
		Address:         firstNonEmptyString(org.LegalAddress, org.PostalAddress),
		INN:             org.INN,
		OGRNIP:          org.OGRN,
		Phone:           org.Phone,
		BankAccount:     org.BankAccount,
		BankName:        org.BankName,
		BankBIK:         org.BankBIK,
		BankCorrAccount: org.BankCorrAccount,
		Position:        signerString("position"),
		LastName:        signerString("last_name"),
		FirstName:       signerString("first_name"),
		MiddleName:      signerString("middle_name"),
	}
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

type MoneyLineInput struct {
	ServiceID string `json:"service_id,omitempty" jsonschema:"existing service id; omit for ad-hoc line"`
	Title     string `json:"title,omitempty" jsonschema:"service title"`
	Unit      string `json:"unit,omitempty" jsonschema:"unit name, defaults to шт"`
	Price     string `json:"price,omitempty" jsonschema:"unit price in rubles, for example 24900.00"`
	Qty       string `json:"qty,omitempty" jsonschema:"quantity, defaults to 1"`
}

type PreviewLine struct {
	ServiceID   string `json:"service_id,omitempty"`
	Title       string `json:"title"`
	Unit        string `json:"unit"`
	Price       string `json:"price"`
	Qty         string `json:"qty"`
	Amount      string `json:"amount"`
	VAT         string `json:"vat"`
	AmountCents int64  `json:"amount_cents"`
}

type ConfirmationResponse struct {
	Status            string         `json:"status"`
	Summary           string         `json:"summary"`
	Preview           map[string]any `json:"preview"`
	Warnings          []string       `json:"warnings"`
	ConfirmationToken string         `json:"confirmation_token"`
	ExpiresAt         time.Time      `json:"expires_at"`
}

type CommitInput struct {
	ConfirmationToken string `json:"confirmation_token" jsonschema:"token returned by prepare_*"`
	IdempotencyKey    string `json:"idempotency_key" jsonschema:"caller-generated idempotency key"`
}

type SearchInput struct {
	Query   string `json:"query,omitempty" jsonschema:"name, INN, KPP, OGRN, phone, email, or document number"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum rows to return"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

type CreateCounterpartyInput struct {
	Type            string `json:"type,omitempty"`
	Name            string `json:"name" jsonschema:"short display name"`
	FullName        string `json:"fullname,omitempty" jsonschema:"full legal name"`
	Address         string `json:"address,omitempty"`
	INN             string `json:"inn" jsonschema:"taxpayer identifier"`
	KPP             string `json:"kpp,omitempty"`
	Phone           string `json:"phone,omitempty"`
	Email           string `json:"email,omitempty"`
	ContactPerson   string `json:"contact_person,omitempty"`
	ContactPosition string `json:"contact_position,omitempty"`
	Comment         string `json:"comment,omitempty"`
	AllowDuplicate  bool   `json:"allow_duplicate,omitempty"`
}

type UpdateCounterpartyInput struct {
	ID              string  `json:"id"`
	Name            *string `json:"name,omitempty"`
	FullName        *string `json:"fullname,omitempty"`
	Address         *string `json:"address,omitempty"`
	INN             *string `json:"inn,omitempty"`
	KPP             *string `json:"kpp,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	Email           *string `json:"email,omitempty"`
	ContactPerson   *string `json:"contact_person,omitempty"`
	ContactPosition *string `json:"contact_position,omitempty"`
	Comment         *string `json:"comment,omitempty"`
}

type IDInput struct {
	ID string `json:"id"`
}

type CreateContractInput struct {
	CounterpartyID    string `json:"counterparty_id,omitempty"`
	CounterpartyQuery string `json:"counterparty_query,omitempty"`
	Number            string `json:"number,omitempty"`
	Date              string `json:"date,omitempty"`
	StartDate         string `json:"start_date,omitempty"`
	EndDate           string `json:"end_date,omitempty"`
	Subject           string `json:"subject,omitempty"`
	Currency          string `json:"currency,omitempty"`
	Status            string `json:"status,omitempty"`
}

type CreateInvoiceInput struct {
	CounterpartyID    string           `json:"counterparty_id,omitempty"`
	CounterpartyQuery string           `json:"counterparty_query,omitempty"`
	ContractID        string           `json:"contract_id,omitempty"`
	ContractNumber    string           `json:"contract_number,omitempty"`
	Date              string           `json:"date,omitempty"`
	Status            string           `json:"status,omitempty"`
	Lines             []MoneyLineInput `json:"lines"`
}

type CreateActInput struct {
	CounterpartyID    string           `json:"counterparty_id,omitempty"`
	CounterpartyQuery string           `json:"counterparty_query,omitempty"`
	ContractID        string           `json:"contract_id,omitempty"`
	ContractNumber    string           `json:"contract_number,omitempty"`
	InvoiceID         string           `json:"invoice_id,omitempty"`
	Date              string           `json:"date,omitempty"`
	Status            string           `json:"status,omitempty"`
	Lines             []MoneyLineInput `json:"lines,omitempty"`
}

type RenderFileInput struct {
	DocumentType string `json:"document_type" jsonschema:"invoice or act"`
	DocumentID   string `json:"document_id"`
}

type UpdateDocumentNumberInput struct {
	DocumentType string `json:"document_type" jsonschema:"invoice or act"`
	DocumentID   string `json:"document_id"`
	Number       string `json:"number" jsonschema:"new numeric document number"`
}

type FileResult struct {
	ID           string `json:"id,omitempty"`
	DocumentType string `json:"document_type"`
	DocumentID   string `json:"document_id"`
	FileKind     string `json:"file_kind"`
	MimeType     string `json:"mime_type"`
	FileName     string `json:"file_name"`
	StoragePath  string `json:"storage_path"`
	SizeBytes    int64  `json:"size_bytes"`
}

type ValidationResult struct {
	Status         string      `json:"status"`
	XMLParse       string      `json:"xml_parse"`
	XSDValidation  string      `json:"xsd_validation"`
	BusinessChecks []string    `json:"business_checks"`
	Warnings       []string    `json:"warnings,omitempty"`
	File           *FileResult `json:"file,omitempty"`
}

func (s *Service) CurrentOrganization(ctx context.Context) (*Organization, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, org_type, full_name, short_name, COALESCE(inn, ''), COALESCE(kpp, ''),
		       COALESCE(ogrn, ''), COALESCE(legal_address, ''), COALESCE(postal_address, ''),
		       COALESCE(phone, ''), COALESCE(bank_account, ''), COALESCE(bank_name, ''),
		       COALESCE(bank_bik, ''), COALESCE(bank_corr_account, ''), tax_regime, vat_mode,
		       timezone, COALESCE(edo_participant_id, ''), signer, numbering_settings, active
		FROM organizations
		WHERE active = true
		ORDER BY created_at
		LIMIT 1
	`)
	var org Organization
	var signerRaw, numberingRaw []byte
	if err := row.Scan(&org.ID, &org.Type, &org.FullName, &org.ShortName, &org.INN, &org.KPP, &org.OGRN,
		&org.LegalAddress, &org.PostalAddress, &org.Phone, &org.BankAccount, &org.BankName, &org.BankBIK,
		&org.BankCorrAccount, &org.TaxRegime, &org.VATMode, &org.Timezone, &org.EDOParticipantID,
		&signerRaw, &numberingRaw, &org.Active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError("ORGANIZATION_NOT_CONFIGURED", "В системе не настроена активная организация-продавец", false, "Обратитесь к администратору для настройки организации")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	_ = json.Unmarshal(signerRaw, &org.Signer)
	_ = json.Unmarshal(numberingRaw, &org.NumberingSettings)
	return &org, nil
}

func (s *Service) SearchCounterparties(ctx context.Context, input SearchInput) (map[string]any, error) {
	limit := boundedLimit(input.Limit, 20)
	query := strings.TrimSpace(input.Query)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, fullname, address, inn, COALESCE(kpp, ''),
		       COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
		       COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
		       COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
		       created_at, updated_at
		FROM customers
		WHERE status <> 'archived'
		  AND ($1 = ''
		       OR name ILIKE $2
		       OR fullname ILIKE $2
		       OR inn ILIKE $2
		       OR COALESCE(kpp, '') ILIKE $2
		       OR COALESCE(phone, '') ILIKE $2
		       OR COALESCE(email, '') ILIKE $2
		       OR COALESCE(contact_person, '') ILIKE $2
		       OR COALESCE(contact_position, '') ILIKE $2
		       OR COALESCE(okpo, '') ILIKE $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, query, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Customer, 0)
	for rows.Next() {
		var c models.Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Fullname, &c.Address, &c.INN, &c.KPP, &c.EDOIDTensor, &c.EDOIDKontur, &c.OKPO, &c.Phone, &c.Email, &c.ContactPerson, &c.ContactPosition, &c.Comment, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return map[string]any{"data": items, "total": len(items)}, rows.Err()
}

// SearchServiceCatalog searches the services table (both the reusable price
// catalog and any ad-hoc services) by name or section, for use when picking
// items for a contract appendix.
func (s *Service) SearchServiceCatalog(ctx context.Context, input SearchInput) (map[string]any, error) {
	limit := boundedLimit(input.Limit, 30)
	query := strings.TrimSpace(input.Query)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, price, unit, section, COALESCE(price_per_hour, 0), COALESCE(hours_per_unit, 0), archived, created_at, updated_at
		FROM services
		WHERE NOT archived
		  AND ($1 = '' OR name ILIKE $2 OR section ILIKE $2)
		ORDER BY section, created_at
		LIMIT $3
	`, query, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Service, 0)
	for rows.Next() {
		var svc models.Service
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.Price, &svc.Unit, &svc.Section, &svc.PricePerHour, &svc.HoursPerUnit, &svc.Archived, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, svc)
	}
	return map[string]any{"data": items, "total": len(items)}, rows.Err()
}

func (s *Service) GetCounterparty(ctx context.Context, input IDInput) (map[string]any, error) {
	c, err := s.db.GetCustomerByID(ctx, input.ID)
	if err != nil {
		return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент не найден", true, "Уточните ID или выполните поиск")
	}
	return map[string]any{"data": c}, nil
}

func (s *Service) PrepareCreateCounterparty(ctx context.Context, input CreateCounterpartyInput) (*ConfirmationResponse, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.INN) == "" {
		return nil, appError("VALIDATION_ERROR", "Название и ИНН обязательны", true, "Передайте name и inn")
	}
	if input.FullName == "" {
		input.FullName = input.Name
	}
	matches, err := s.findCounterpartyDuplicates(ctx, input.INN, input.KPP)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	if len(matches) > 0 && !input.AllowDuplicate {
		warnings = append(warnings, "Найден возможный дубль. Новая карточка не будет создана без allow_duplicate=true.")
	}
	payload := map[string]any{"input": input}
	preview := map[string]any{"counterparty": input, "duplicates": matches}
	if len(matches) > 0 && !input.AllowDuplicate {
		preview["blocked"] = true
	}
	return s.createConfirmation(ctx, "counterparties.commit_create", UserFromContext(ctx), payload, preview, warnings, "Будет создан контрагент "+input.Name)
}

func (s *Service) CommitCreateCounterparty(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input CreateCounterpartyInput `json:"input"`
	}
	return s.commitWithConfirmation(ctx, "counterparties.commit_create", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		matches, err := s.findCounterpartyDuplicatesTx(ctx, tx, payload.Input.INN, payload.Input.KPP)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 && !payload.Input.AllowDuplicate {
			return nil, appError("COUNTERPARTY_DUPLICATE", "Контрагент с такими реквизитами уже существует", true, "Используйте существующую карточку или повторите prepare с allow_duplicate=true")
		}
		var c models.Customer
		err = tx.QueryRowContext(ctx, `
			INSERT INTO customers (name, fullname, address, inn, kpp, phone, email, contact_person, contact_position, comment, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active')
			RETURNING id, name, fullname, address, inn, COALESCE(kpp, ''),
			          COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
			          COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
			          COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
			          created_at, updated_at
		`, payload.Input.Name, payload.Input.FullName, payload.Input.Address, payload.Input.INN, payload.Input.KPP,
			payload.Input.Phone, payload.Input.Email, payload.Input.ContactPerson, payload.Input.ContactPosition, payload.Input.Comment).
			Scan(&c.ID, &c.Name, &c.Fullname, &c.Address, &c.INN, &c.KPP, &c.EDOIDTensor, &c.EDOIDKontur, &c.OKPO, &c.Phone, &c.Email, &c.ContactPerson, &c.ContactPosition, &c.Comment, &c.Status, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "created", "data": c}, nil
	})
}

func (s *Service) PrepareUpdateCounterparty(ctx context.Context, input UpdateCounterpartyInput) (*ConfirmationResponse, error) {
	current, err := s.db.GetCustomerByID(ctx, input.ID)
	if err != nil {
		return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент не найден", true, "Уточните ID")
	}
	payload := map[string]any{"input": input}
	preview := map[string]any{"current": current, "changes": input}
	return s.createConfirmation(ctx, "counterparties.commit_update", UserFromContext(ctx), payload, preview, nil, "Будет обновлен контрагент "+current.Name)
}

func (s *Service) CommitUpdateCounterparty(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input UpdateCounterpartyInput `json:"input"`
	}
	return s.commitWithConfirmation(ctx, "counterparties.commit_update", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		var c models.Customer
		err := tx.QueryRowContext(ctx, `
			UPDATE customers
			SET name = COALESCE($2, name),
			    fullname = COALESCE($3, fullname),
			    address = COALESCE($4, address),
			    inn = COALESCE($5, inn),
			    kpp = COALESCE($6, kpp),
			    phone = COALESCE($7, phone),
			    email = COALESCE($8, email),
			    contact_person = COALESCE($9, contact_person),
			    contact_position = COALESCE($10, contact_position),
			    comment = COALESCE($11, comment)
			WHERE id = $1
			RETURNING id, name, fullname, address, inn, COALESCE(kpp, ''),
			          COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
			          COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
			          COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
			          created_at, updated_at
		`, inputID(payload.Input.ID), payload.Input.Name, payload.Input.FullName, payload.Input.Address, payload.Input.INN, payload.Input.KPP,
			payload.Input.Phone, payload.Input.Email, payload.Input.ContactPerson, payload.Input.ContactPosition, payload.Input.Comment).
			Scan(&c.ID, &c.Name, &c.Fullname, &c.Address, &c.INN, &c.KPP, &c.EDOIDTensor, &c.EDOIDKontur, &c.OKPO, &c.Phone, &c.Email, &c.ContactPerson, &c.ContactPosition, &c.Comment, &c.Status, &c.CreatedAt, &c.UpdatedAt)
		if err == sql.ErrNoRows {
			return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент не найден", true, "Уточните ID")
		}
		return map[string]any{"status": "updated", "data": c}, err
	})
}

func (s *Service) ArchiveCounterparty(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "counterparties.archive", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		var linked int
		if err := tx.QueryRowContext(ctx, `
			SELECT (SELECT COUNT(*) FROM invoices WHERE customer_id = $1)
			     + (SELECT COUNT(*) FROM contracts WHERE customer_id = $1)
		`, payload.ID).Scan(&linked); err != nil {
			return nil, err
		}
		if linked > 0 {
			return nil, appError("FORBIDDEN", "У контрагента есть связанные документы или договоры", true, "Архивируйте документы либо оставьте карточку активной")
		}
		res, err := tx.ExecContext(ctx, `UPDATE customers SET status='archived', archived_at=CURRENT_TIMESTAMP WHERE id=$1`, payload.ID)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент не найден", true, "Уточните ID")
		}
		return map[string]any{"status": "archived", "id": payload.ID}, nil
	})
}

func (s *Service) PrepareArchiveCounterparty(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	c, err := s.db.GetCustomerByID(ctx, input.ID)
	if err != nil {
		return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "counterparties.archive", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"counterparty": c}, nil, "Будет архивирован контрагент "+c.Name)
}

func (s *Service) ListCounterpartyDocuments(ctx context.Context, input IDInput) (map[string]any, error) {
	invoices, invoiceTotal, err := s.db.GetInvoices(ctx, input.ID, "", nil, 1, 200)
	if err != nil {
		return nil, err
	}
	acts, actTotal, err := s.db.GetActs(ctx, input.ID, "", nil, 1, 200)
	if err != nil {
		return nil, err
	}
	return map[string]any{"invoices": invoices, "acts": acts, "total": invoiceTotal + actTotal}, nil
}

func (s *Service) ListCounterpartyContracts(ctx context.Context, input IDInput) (map[string]any, error) {
	contracts, total, err := s.db.GetContracts(ctx, input.ID, 1, 200)
	if err != nil {
		return nil, err
	}
	return map[string]any{"data": contracts, "total": total}, nil
}

func (s *Service) SearchContracts(ctx context.Context, input SearchInput) (map[string]any, error) {
	limit := boundedLimit(input.Limit, 20)
	query := strings.TrimSpace(input.Query)
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.customer_id, c.number, c.currency, c.status, c.topic,
		       COALESCE(c.start_date::text, ''), COALESCE(c.end_date::text, ''),
		       c.created_at, c.updated_at
		FROM contracts c
		JOIN customers cu ON cu.id = c.customer_id
		WHERE c.status <> 'canceled'
		  AND ($1 = '' OR c.number ILIKE $2 OR c.topic ILIKE $2 OR cu.name ILIKE $2 OR cu.fullname ILIKE $2 OR cu.inn ILIKE $2)
		ORDER BY c.created_at DESC
		LIMIT $3
	`, query, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Contract, 0)
	for rows.Next() {
		var c models.Contract
		if err := rows.Scan(&c.ID, &c.CustomerID, &c.Number, &c.Currency, &c.Status, &c.Topic, &c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return map[string]any{"data": items, "total": len(items)}, rows.Err()
}

func (s *Service) GetContract(ctx context.Context, input IDInput) (map[string]any, error) {
	contract, err := s.db.GetContractByID(ctx, input.ID)
	if err != nil {
		return nil, appError("CONTRACT_NOT_FOUND", "Договор не найден", true, "Уточните ID или выполните поиск")
	}
	return map[string]any{"data": contract}, nil
}

func (s *Service) PrepareCreateContract(ctx context.Context, input CreateContractInput) (*ConfirmationResponse, error) {
	customer, err := s.resolveCounterparty(ctx, input.CounterpartyID, input.CounterpartyQuery)
	if err != nil {
		return nil, err
	}
	if input.Number == "" {
		next, err := s.db.GetNextContractNumber(ctx, customer.ID)
		if err != nil {
			return nil, err
		}
		input.Number = strconv.FormatInt(next, 10)
	}
	if input.Currency == "" {
		input.Currency = "RUB"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	topic, err := normalizeContractTopic(input.Subject)
	if err != nil {
		return nil, err
	}
	input.Subject = topic
	input.CounterpartyID = customer.ID
	input.CounterpartyQuery = ""
	return s.createConfirmation(ctx, "contracts.commit_create", UserFromContext(ctx), map[string]any{"input": input}, map[string]any{"counterparty": customer, "contract": input}, nil, "Будет создан договор "+input.Number+" для "+customer.Name)
}

func (s *Service) CommitCreateContract(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input CreateContractInput `json:"input"`
	}
	return s.commitWithConfirmation(ctx, "contracts.commit_create", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		topic, err := normalizeContractTopic(payload.Input.Subject)
		if err != nil {
			return nil, err
		}
		payload.Input.Subject = topic
		var c models.Contract
		err = tx.QueryRowContext(ctx, `
			INSERT INTO contracts (customer_id, number, currency, status, topic, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::date, NULLIF($7, '')::date)
			RETURNING id, customer_id, number, currency, status, topic,
			          COALESCE(start_date::text, ''), COALESCE(end_date::text, ''), created_at, updated_at
		`, payload.Input.CounterpartyID, payload.Input.Number, payload.Input.Currency, payload.Input.Status, payload.Input.Subject, payload.Input.StartDate, payload.Input.EndDate).
			Scan(&c.ID, &c.CustomerID, &c.Number, &c.Currency, &c.Status, &c.Topic, &c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, appError("DOCUMENT_DUPLICATE", "Договор с таким номером уже существует", true, "Выберите существующий договор или другой номер")
			}
			if isForeignKeyViolation(err) {
				return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент для договора не найден", true, "Проверьте counterparty_id")
			}
			return nil, err
		}
		return map[string]any{"status": "created", "data": c}, nil
	})
}

// ContractAppendixLineInput represents one line of a "Приложение к договору"
// (contract appendix / смета): either a reference to a catalog service
// (ServiceID), optionally overriding its title/unit/price/section, or a
// fully ad-hoc line.
type ContractAppendixLineInput struct {
	ServiceID string `json:"service_id,omitempty" jsonschema:"existing catalog service id from services.search; omit for an ad-hoc line"`
	Section   string `json:"section,omitempty" jsonschema:"section label shown on the printed appendix; inherited from the catalog service if omitted"`
	Title     string `json:"title,omitempty" jsonschema:"line title; required for ad-hoc lines, overrides the catalog name if service_id is set"`
	Unit      string `json:"unit,omitempty" jsonschema:"unit name, defaults to услуга"`
	Price     string `json:"price,omitempty" jsonschema:"unit price in rubles, for example 24900.00; overrides the catalog price if service_id is set"`
	Qty       string `json:"qty,omitempty" jsonschema:"quantity, defaults to 1"`
}

// CreateContractAppendixInput is the input for preparing a new contract appendix.
type CreateContractAppendixInput struct {
	CounterpartyID    string                      `json:"counterparty_id,omitempty"`
	CounterpartyQuery string                      `json:"counterparty_query,omitempty"`
	ContractID        string                      `json:"contract_id,omitempty"`
	ContractNumber    string                      `json:"contract_number,omitempty"`
	Number            string                      `json:"number,omitempty" jsonschema:"appendix number; auto-assigned per contract if omitted"`
	Date              string                      `json:"date,omitempty" jsonschema:"DD.MM.YYYY, defaults to today"`
	Lines             []ContractAppendixLineInput `json:"lines"`
}

type appendixDBLine struct {
	ServiceID string
	Section   string
	Title     string
	Unit      string
	Price     string
	Qty       string
	Amount    string
}

func (s *Service) buildAppendixLinesTx(ctx context.Context, tx *sql.Tx, inputs []ContractAppendixLineInput) ([]appendixDBLine, int64, error) {
	lines := make([]appendixDBLine, 0, len(inputs))
	var total int64
	for _, input := range inputs {
		title := strings.TrimSpace(input.Title)
		unit := strings.TrimSpace(input.Unit)
		price := strings.TrimSpace(input.Price)
		section := strings.TrimSpace(input.Section)
		serviceID := strings.TrimSpace(input.ServiceID)
		qty := strings.TrimSpace(input.Qty)
		if qty == "" {
			qty = "1"
		}

		if serviceID != "" {
			var catalogName, catalogPrice, catalogUnit, catalogSection string
			if err := tx.QueryRowContext(ctx, `SELECT name, price::text, unit, section FROM services WHERE id=$1`, serviceID).
				Scan(&catalogName, &catalogPrice, &catalogUnit, &catalogSection); err != nil {
				return nil, 0, appError("VALIDATION_ERROR", "Услуга не найдена", true, "Проверьте service_id")
			}
			if title == "" {
				title = catalogName
			}
			if price == "" {
				price = catalogPrice
			}
			if unit == "" {
				unit = catalogUnit
			}
			if section == "" {
				section = catalogSection
			}
		}
		if unit == "" {
			unit = "услуга"
		}
		if title == "" || price == "" {
			return nil, 0, appError("VALIDATION_ERROR", "У строки должны быть title и price", true, "Исправьте строки приложения")
		}
		priceCents, err := parseMoneyCents(price)
		if err != nil {
			return nil, 0, appError("VALIDATION_ERROR", "Некорректная цена: "+price, true, "Передайте цену в рублях")
		}
		qtyMilli, err := parseQtyMilli(qty)
		if err != nil || qtyMilli <= 0 {
			return nil, 0, appError("VALIDATION_ERROR", "Некорректное количество: "+qty, true, "Передайте положительное количество")
		}
		amountCents := (priceCents*qtyMilli + 500) / 1000
		total += amountCents
		lines = append(lines, appendixDBLine{
			ServiceID: serviceID, Section: section, Title: title, Unit: unit,
			Price: decimalFromCents(priceCents), Qty: decimalFromMilli(qtyMilli), Amount: decimalFromCents(amountCents),
		})
	}
	return lines, total, nil
}

// PrepareCreateContractAppendix prepares a new "Приложение к договору"
// (contract appendix / смета) — a standalone printable document listing the
// specific work items (catalog or ad-hoc) agreed for a contract.
func (s *Service) PrepareCreateContractAppendix(ctx context.Context, input CreateContractAppendixInput) (*ConfirmationResponse, error) {
	var customer *models.Customer
	var err error
	if input.CounterpartyID != "" || input.CounterpartyQuery != "" {
		customer, err = s.resolveCounterparty(ctx, input.CounterpartyID, input.CounterpartyQuery)
		if err != nil {
			return nil, err
		}
	}
	customerID := ""
	if customer != nil {
		customerID = customer.ID
	}
	contract, err := s.resolveContract(ctx, customerID, input.ContractID, input.ContractNumber)
	if err != nil {
		return nil, err
	}
	if input.Number == "" {
		next, err := s.db.GetNextContractAppendixNumber(ctx, contract.ID)
		if err != nil {
			return nil, err
		}
		input.Number = strconv.FormatInt(next, 10)
	}
	if input.Date == "" {
		input.Date = todayRu()
	}
	if _, err := time.Parse("02.01.2006", input.Date); err != nil {
		return nil, appError("VALIDATION_ERROR", "Дата приложения должна быть в формате DD.MM.YYYY", true, "Исправьте дату")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	lines, totalCents, err := s.buildAppendixLinesTx(ctx, tx, input.Lines)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, appError("VALIDATION_ERROR", "Нужна хотя бы одна строка приложения", true, "Передайте lines")
	}

	input.ContractID = contract.ID
	input.ContractNumber = ""
	input.CounterpartyID = ""
	input.CounterpartyQuery = ""

	preview := map[string]any{"contract": contract, "number": input.Number, "date": input.Date, "lines": lines, "total": formatMoney(totalCents)}
	summary := "Будет создано приложение №" + input.Number + " к договору " + contract.Number + " на " + formatMoney(totalCents)
	return s.createConfirmation(ctx, "contract_appendices.commit_create", UserFromContext(ctx), map[string]any{"input": input, "total_cents": totalCents}, preview, nil, summary)
}

// CommitCreateContractAppendix commits a previously prepared contract appendix.
func (s *Service) CommitCreateContractAppendix(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input      CreateContractAppendixInput `json:"input"`
		TotalCents int64                       `json:"total_cents"`
	}
	return s.commitWithConfirmation(ctx, "contract_appendices.commit_create", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		lines, totalCents, err := s.buildAppendixLinesTx(ctx, tx, payload.Input.Lines)
		if err != nil {
			return nil, err
		}
		if totalCents != payload.TotalCents {
			return nil, appError("CONFIRMATION_MISMATCH", "Сумма изменилась после подготовки", true, "Повторите prepare_create")
		}

		var appendixID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO contract_appendices (contract_id, number, date, status, total_amount)
			VALUES ($1, $2, $3, 'draft', $4)
			RETURNING id
		`, payload.Input.ContractID, payload.Input.Number, payload.Input.Date, decimalFromCents(totalCents)).Scan(&appendixID)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, appError("DOCUMENT_DUPLICATE", "Приложение с таким номером уже существует для этого договора", true, "Выберите другой номер")
			}
			if isForeignKeyViolation(err) {
				return nil, appError("CONTRACT_NOT_FOUND", "Договор не найден", true, "Проверьте contract_id")
			}
			return nil, err
		}

		for i, line := range lines {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO contract_appendix_lines
					(appendix_id, service_id, section, position, title_snapshot, unit_snapshot, price_snapshot, qty, amount)
				VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7::numeric, $8::numeric, $9::numeric)
			`, appendixID, line.ServiceID, line.Section, i+1, line.Title, line.Unit, line.Price, line.Qty, line.Amount); err != nil {
				return nil, fmt.Errorf("failed to create appendix line: %w", err)
			}
		}

		return map[string]any{"status": "created", "id": appendixID, "number": payload.Input.Number, "total": formatMoney(totalCents)}, nil
	})
}

func (s *Service) ArchiveContract(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "contracts.archive", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		res, err := tx.ExecContext(ctx, `UPDATE contracts SET status='canceled' WHERE id=$1`, payload.ID)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, appError("CONTRACT_NOT_FOUND", "Договор не найден", true, "Уточните ID")
		}
		return map[string]any{"status": "archived", "id": payload.ID}, nil
	})
}

func (s *Service) PrepareArchiveContract(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	contract, err := s.db.GetContractByID(ctx, input.ID)
	if err != nil {
		return nil, appError("CONTRACT_NOT_FOUND", "Договор не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "contracts.archive", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"contract": contract}, nil, "Будет архивирован договор "+contract.Number)
}

func (s *Service) ListContractDocuments(ctx context.Context, input IDInput) (map[string]any, error) {
	invoices, invoiceTotal, err := s.db.GetInvoices(ctx, "", input.ID, nil, 1, 200)
	if err != nil {
		return nil, err
	}
	acts, actTotal, err := s.db.GetActs(ctx, "", input.ID, nil, 1, 200)
	if err != nil {
		return nil, err
	}
	return map[string]any{"invoices": invoices, "acts": acts, "total": invoiceTotal + actTotal}, nil
}

func (s *Service) SearchInvoices(ctx context.Context, input SearchInput) (map[string]any, error) {
	page, perPage := paging(input)
	invoices, total, err := s.db.GetInvoices(ctx, "", "", nil, page, perPage)
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	if query != "" {
		filtered := invoices[:0]
		for _, invoice := range invoices {
			if strings.Contains(strings.ToLower(invoice.Number), query) || strings.Contains(strings.ToLower(invoice.ContractNumber), query) {
				filtered = append(filtered, invoice)
			}
		}
		invoices = filtered
		total = len(filtered)
	}
	return map[string]any{"data": invoices, "total": total, "page": page, "per_page": perPage}, nil
}

func (s *Service) GetInvoice(ctx context.Context, input IDInput) (map[string]any, error) {
	invoice, err := s.db.GetInvoiceWithServices(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните ID")
	}
	return map[string]any{"data": invoice}, nil
}

func (s *Service) PrepareCreateInvoice(ctx context.Context, input CreateInvoiceInput) (*ConfirmationResponse, error) {
	customer, contract, lines, totalCents, warnings, err := s.prepareDocument(ctx, input.CounterpartyID, input.CounterpartyQuery, input.ContractID, input.ContractNumber, input.Lines)
	if err != nil {
		return nil, err
	}
	if input.Date == "" {
		input.Date = todayRu()
	}
	if _, err := time.Parse("02.01.2006", input.Date); err != nil {
		return nil, appError("VALIDATION_ERROR", "Дата счёта должна быть в формате DD.MM.YYYY", true, "Исправьте дату")
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	input.CounterpartyID = customer.ID
	input.CounterpartyQuery = ""
	input.ContractID = contract.ID
	input.ContractNumber = contract.Number
	dupes, err := s.findInvoiceDuplicates(ctx, contract.ID, input.Date, totalCents)
	if err != nil {
		return nil, err
	}
	if len(dupes) > 0 {
		warnings = append(warnings, "Найдены возможные дубли счёта за эту дату и сумму.")
	}
	preview := map[string]any{"counterparty": customer, "contract": contract, "date": input.Date, "status": input.Status, "lines": lines, "total": formatMoney(totalCents), "vat": "без НДС", "possible_duplicates": dupes}
	return s.createConfirmation(ctx, "invoices.commit_create", UserFromContext(ctx), map[string]any{"input": input, "total_cents": totalCents}, preview, warnings, "Будет создан счёт без НДС на "+formatMoney(totalCents))
}

func (s *Service) CommitCreateInvoice(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input      CreateInvoiceInput `json:"input"`
		TotalCents int64              `json:"total_cents"`
	}
	return s.commitWithConfirmation(ctx, "invoices.commit_create", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		org, err := s.currentOrganizationTx(ctx, tx)
		if err != nil {
			return nil, err
		}
		number, err := s.nextNumberTx(ctx, tx, org.ID, "invoice")
		if err != nil {
			return nil, err
		}
		lines, totalCents, err := s.buildLinesTx(ctx, tx, payload.Input.Lines)
		if err != nil {
			return nil, err
		}
		if totalCents != payload.TotalCents {
			return nil, appError("CONFIRMATION_MISMATCH", "Сумма изменилась после подготовки", true, "Повторите prepare_create")
		}
		var invoice models.Invoice
		err = tx.QueryRowContext(ctx, `
			INSERT INTO invoices (contract_id, customer_id, number, date, status, total_amount, archived, contract_number)
			VALUES ($1, $2, $3, $4, $5, $6::numeric, false, $7)
			RETURNING id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
		`, payload.Input.ContractID, payload.Input.CounterpartyID, strconv.FormatInt(number, 10), payload.Input.Date, payload.Input.Status, decimalFromCents(totalCents), payload.Input.ContractNumber).
			Scan(&invoice.ID, &invoice.ContractID, &invoice.CustomerID, &invoice.Number, &invoice.Date, &invoice.Status, &invoice.TotalAmount, &invoice.Archived, &invoice.ContractNumber, &invoice.CreatedAt, &invoice.UpdatedAt)
		if err != nil {
			if isForeignKeyViolation(err) {
				return nil, appError("CONTRACT_NOT_FOUND", "Договор или контрагент для счёта не найден", true, "Проверьте contract_id и counterparty_id")
			}
			return nil, err
		}
		for _, line := range lines {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO invoice_lines (invoice_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
				VALUES ($1, NULLIF($2, '')::uuid, $3, $4, 0, $5::numeric, $6::numeric, $7::numeric)
			`, invoice.ID, line.ServiceID, line.Title, line.Unit, line.Price, line.Qty, line.Amount)
			if err != nil {
				return nil, err
			}
		}
		return map[string]any{"status": "created", "data": invoice, "vat": "без НДС"}, nil
	})
}

func (s *Service) PrepareIssueInvoice(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	invoice, err := s.db.GetInvoiceByID(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "invoices.commit_issue", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"invoice": invoice, "next_status": "issued"}, nil, "Счёт будет выставлен")
}

func (s *Service) CommitIssueInvoice(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "invoices.commit_issue", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		invoice, err := updateInvoiceStatusTx(ctx, tx, payload.ID, "issued")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "issued", "data": invoice}, nil
	})
}

func (s *Service) MarkInvoicePaid(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "invoices.mark_paid", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		invoice, err := updateInvoiceStatusTx(ctx, tx, payload.ID, "paid")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "paid", "payment_status_source": "manual", "data": invoice}, nil
	})
}

func (s *Service) PrepareMarkInvoicePaid(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	invoice, err := s.db.GetInvoiceByID(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "invoices.mark_paid", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"invoice": invoice, "payment_status_source": "manual"}, nil, "Счёт будет отмечен оплаченным вручную")
}

func (s *Service) ListUnpaidInvoices(ctx context.Context, input SearchInput) (map[string]any, error) {
	limit := boundedLimit(input.Limit, 100)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
		FROM invoices
		WHERE archived = false AND status IN ('draft', 'issued')
		ORDER BY to_date(date, 'DD.MM.YYYY') DESC, number DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Invoice
	for rows.Next() {
		var invoice models.Invoice
		if err := rows.Scan(&invoice.ID, &invoice.ContractID, &invoice.CustomerID, &invoice.Number, &invoice.Date, &invoice.Status, &invoice.TotalAmount, &invoice.Archived, &invoice.ContractNumber, &invoice.CreatedAt, &invoice.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, invoice)
	}
	return map[string]any{"data": items, "total": len(items), "payment_status_source": "manual"}, rows.Err()
}

func (s *Service) SearchActs(ctx context.Context, input SearchInput) (map[string]any, error) {
	page, perPage := paging(input)
	acts, total, err := s.db.GetActs(ctx, "", "", nil, page, perPage)
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	if query != "" {
		filtered := acts[:0]
		for _, act := range acts {
			if strings.Contains(strings.ToLower(act.Number), query) || strings.Contains(strings.ToLower(act.ContractNumber), query) {
				filtered = append(filtered, act)
			}
		}
		acts = filtered
		total = len(filtered)
	}
	return map[string]any{"data": acts, "total": total, "page": page, "per_page": perPage}, nil
}

func (s *Service) GetAct(ctx context.Context, input IDInput) (map[string]any, error) {
	act, err := s.db.GetActWithServices(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните ID")
	}
	return map[string]any{"data": act}, nil
}

func (s *Service) PrepareCreateAct(ctx context.Context, input CreateActInput) (*ConfirmationResponse, error) {
	if input.Date == "" {
		input.Date = todayRu()
	}
	if _, err := time.Parse("02.01.2006", input.Date); err != nil {
		return nil, appError("VALIDATION_ERROR", "Дата акта должна быть в формате DD.MM.YYYY", true, "Исправьте дату")
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	var preview map[string]any
	var payload map[string]any
	var warnings []string
	if input.InvoiceID != "" {
		invoice, err := s.db.GetInvoiceWithServices(ctx, input.InvoiceID)
		if err != nil {
			return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните invoice_id")
		}
		customer, err := s.db.GetCustomerByID(ctx, invoice.CustomerID)
		if err != nil {
			return nil, err
		}
		contract, err := s.db.GetContractByID(ctx, invoice.ContractID)
		if err != nil {
			return nil, err
		}
		input.CounterpartyID = invoice.CustomerID
		input.ContractID = invoice.ContractID
		input.ContractNumber = invoice.ContractNumber
		totalCents := centsFromFloat(invoice.TotalAmount)
		preview = map[string]any{"source_invoice": invoice, "counterparty": customer, "contract": contract, "date": input.Date, "total": formatMoney(totalCents), "vat": "без НДС"}
		payload = map[string]any{"input": input, "total_cents": totalCents}
	} else {
		customer, contract, lines, totalCents, w, err := s.prepareDocument(ctx, input.CounterpartyID, input.CounterpartyQuery, input.ContractID, input.ContractNumber, input.Lines)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, w...)
		input.CounterpartyID = customer.ID
		input.CounterpartyQuery = ""
		input.ContractID = contract.ID
		input.ContractNumber = contract.Number
		preview = map[string]any{"counterparty": customer, "contract": contract, "date": input.Date, "lines": lines, "total": formatMoney(totalCents), "vat": "без НДС"}
		payload = map[string]any{"input": input, "total_cents": totalCents}
	}
	return s.createConfirmation(ctx, "acts.commit_create", UserFromContext(ctx), payload, preview, warnings, "Будет создан акт без НДС")
}

func (s *Service) CommitCreateAct(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input      CreateActInput `json:"input"`
		TotalCents int64          `json:"total_cents"`
	}
	return s.commitWithConfirmation(ctx, "acts.commit_create", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		org, err := s.currentOrganizationTx(ctx, tx)
		if err != nil {
			return nil, err
		}
		number, err := s.nextNumberTx(ctx, tx, org.ID, "act")
		if err != nil {
			return nil, err
		}
		if payload.Input.InvoiceID != "" {
			act, err := createActFromInvoiceTx(ctx, tx, payload.Input.InvoiceID, strconv.FormatInt(number, 10), payload.Input.Date, payload.Input.Status)
			if err != nil {
				return nil, err
			}
			return map[string]any{"status": "created", "data": act, "vat": "без НДС"}, nil
		}
		lines, totalCents, err := s.buildLinesTx(ctx, tx, payload.Input.Lines)
		if err != nil {
			return nil, err
		}
		if totalCents != payload.TotalCents {
			return nil, appError("CONFIRMATION_MISMATCH", "Сумма изменилась после подготовки", true, "Повторите prepare_create")
		}
		var act models.Act
		err = tx.QueryRowContext(ctx, `
			INSERT INTO acts (contract_id, number, date, status, total_amount, archived)
			VALUES ($1, $2, $3, $4, $5::numeric, false)
			RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
		`, payload.Input.ContractID, strconv.FormatInt(number, 10), payload.Input.Date, payload.Input.Status, decimalFromCents(totalCents)).
			Scan(&act.ID, &act.ContractID, &act.Number, &act.Date, &act.Status, &act.TotalAmount, &act.Archived, &act.CreatedAt, &act.UpdatedAt)
		if err != nil {
			if isForeignKeyViolation(err) {
				return nil, appError("CONTRACT_NOT_FOUND", "Договор для акта не найден", true, "Проверьте contract_id")
			}
			return nil, err
		}
		for _, line := range lines {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO act_lines (act_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
				VALUES ($1, NULLIF($2, '')::uuid, $3, $4, 0, $5::numeric, $6::numeric, $7::numeric)
			`, act.ID, line.ServiceID, line.Title, line.Unit, line.Price, line.Qty, line.Amount)
			if err != nil {
				return nil, err
			}
		}
		act.CustomerID = payload.Input.CounterpartyID
		act.ContractNumber = payload.Input.ContractNumber
		return map[string]any{"status": "created", "data": act, "vat": "без НДС"}, nil
	})
}

func (s *Service) PrepareIssueAct(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	act, err := s.db.GetActByID(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "acts.commit_issue", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"act": act, "next_status": "signed"}, nil, "Акт будет проведён")
}

func (s *Service) CommitIssueAct(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "acts.commit_issue", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		act, err := updateActStatusTx(ctx, tx, payload.ID, "signed")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "signed", "data": act}, nil
	})
}

func (s *Service) CancelInvoice(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "invoices.cancel", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		invoice, err := updateInvoiceStatusTx(ctx, tx, payload.ID, "canceled")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "cancelled", "data": invoice}, nil
	})
}

func (s *Service) PrepareCancelInvoice(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	invoice, err := s.db.GetInvoiceByID(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "invoices.cancel", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"invoice": invoice}, []string{"Отмена меняет статус документа."}, "Счёт будет отменён")
}

func (s *Service) CancelAct(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		ID string `json:"id"`
	}
	return s.commitWithConfirmation(ctx, "acts.cancel", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		act, err := updateActStatusTx(ctx, tx, payload.ID, "canceled")
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "cancelled", "data": act}, nil
	})
}

func (s *Service) PrepareCancelAct(ctx context.Context, input IDInput) (*ConfirmationResponse, error) {
	act, err := s.db.GetActByID(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните ID")
	}
	return s.createConfirmation(ctx, "acts.cancel", UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"act": act}, []string{"Отмена меняет статус документа."}, "Акт будет отменён")
}

func (s *Service) PrepareUpdateDocumentNumber(ctx context.Context, input UpdateDocumentNumberInput) (*ConfirmationResponse, error) {
	docType := normalizeDocumentType(input.DocumentType)
	number := strings.TrimSpace(input.Number)
	if docType == "" {
		return nil, appError("VALIDATION_ERROR", "document_type должен быть invoice или act", true, "Передайте document_type: invoice или act")
	}
	if !numericString(number) {
		return nil, appError("VALIDATION_ERROR", "Номер документа должен быть числом", true, "Передайте number только цифрами")
	}
	input.DocumentType = docType
	input.Number = number

	current, duplicate, err := s.documentNumberPreview(ctx, docType, input.DocumentID, number)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	if duplicate != nil {
		warnings = append(warnings, "Номер уже занят другим документом в этом договоре. Commit будет отклонен.")
	}
	preview := map[string]any{
		"document_type": docType,
		"current":       current,
		"new_number":    number,
	}
	if duplicate != nil {
		preview["duplicate"] = duplicate
	}
	return s.createConfirmation(ctx, "documents.commit_update_number", UserFromContext(ctx), map[string]any{"input": input}, preview, warnings, "Будет изменен номер документа на "+number)
}

func (s *Service) CommitUpdateDocumentNumber(ctx context.Context, input CommitInput) (map[string]any, error) {
	var payload struct {
		Input UpdateDocumentNumberInput `json:"input"`
	}
	return s.commitWithConfirmation(ctx, "documents.commit_update_number", input, &payload, func(tx *sql.Tx) (map[string]any, error) {
		docType := normalizeDocumentType(payload.Input.DocumentType)
		number := strings.TrimSpace(payload.Input.Number)
		if docType == "" || !numericString(number) {
			return nil, appError("VALIDATION_ERROR", "Некорректный тип или номер документа", true, "Повторите prepare_update_number")
		}
		if docType == "invoice" {
			invoice, err := updateInvoiceNumberTx(ctx, tx, payload.Input.DocumentID, number)
			if err != nil {
				return nil, err
			}
			if parsed, err := strconv.ParseInt(number, 10, 64); err == nil {
				if err := s.bumpNumberSequenceTx(ctx, tx, "invoice", parsed); err != nil {
					return nil, err
				}
			}
			return map[string]any{"status": "updated", "document_type": "invoice", "data": invoice}, nil
		}

		act, err := updateActNumberTx(ctx, tx, payload.Input.DocumentID, number)
		if err != nil {
			return nil, err
		}
		if parsed, err := strconv.ParseInt(number, 10, 64); err == nil {
			if err := s.bumpNumberSequenceTx(ctx, tx, "act", parsed); err != nil {
				return nil, err
			}
		}
		return map[string]any{"status": "updated", "document_type": "act", "data": act}, nil
	})
}

func (s *Service) RenderPDF(ctx context.Context, input RenderFileInput) (*FileResult, error) {
	docType := normalizeDocumentType(input.DocumentType)
	if docType == "" {
		return nil, appError("VALIDATION_ERROR", "document_type должен быть invoice или act", true, "Исправьте тип документа")
	}
	org, err := s.CurrentOrganization(ctx)
	if err != nil {
		return nil, err
	}
	var title string
	var lines []string
	var docDate string
	if docType == "invoice" {
		invoice, err := s.db.GetInvoiceWithServices(ctx, input.DocumentID)
		if err != nil {
			return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните document_id")
		}
		customer, _ := s.db.GetCustomerByID(ctx, invoice.CustomerID)
		title = "Invoice " + invoice.Number
		docDate = invoice.Date
		lines = append(lines, "Счет N "+invoice.Number+" от "+invoice.Date)
		if customer != nil {
			lines = append(lines, "Покупатель: "+customer.Name)
		}
		lines = append(lines, "Продавец: "+org.ShortName, "НДС: без НДС", "Итого: "+formatMoney(centsFromFloat(invoice.TotalAmount)))
		for _, item := range invoice.Services {
			lines = append(lines, item.Name+" - "+formatMoney(centsFromFloat(item.Amount)))
		}
	} else {
		act, err := s.db.GetActWithServices(ctx, input.DocumentID)
		if err != nil {
			return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните document_id")
		}
		customer, _ := s.db.GetCustomerByID(ctx, act.CustomerID)
		title = "Act " + act.Number
		docDate = act.Date
		lines = append(lines, "Акт N "+act.Number+" от "+act.Date)
		if customer != nil {
			lines = append(lines, "Заказчик: "+customer.Name)
		}
		lines = append(lines, "Исполнитель: "+org.ShortName, "НДС: без НДС", "Итого: "+formatMoney(centsFromFloat(act.TotalAmount)))
		for _, item := range act.Services {
			lines = append(lines, item.Name+" - "+formatMoney(centsFromFloat(item.Amount)))
		}
	}
	data := minimalPDF(title, lines)
	path, filename, err := s.documentPath(org.ID, docDate, docType, input.DocumentID, docType+"-"+input.DocumentID+".pdf")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return nil, err
	}
	result, err := s.upsertDocumentFile(ctx, org.ID, docType, input.DocumentID, "pdf", "application/pdf", path, filename, int64(len(data)))
	if err != nil {
		_ = os.Remove(path)
		return nil, appError("STORAGE_ERROR", "Файл сформирован, но не удалось сохранить запись о нём — повторите запрос", true, "Повторите render_pdf")
	}
	return result, nil
}

func (s *Service) ExportActUPDXML(ctx context.Context, input IDInput) (*FileResult, error) {
	org, err := s.CurrentOrganization(ctx)
	if err != nil {
		return nil, err
	}
	act, err := s.db.GetActWithServices(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните ID")
	}
	customer, err := s.db.GetCustomerByID(ctx, act.CustomerID)
	if err != nil {
		return nil, err
	}
	contract, err := s.db.GetContractByID(ctx, act.ContractID)
	if err != nil {
		return nil, err
	}
	data, filename, err := updxml.BuildActUPDXML(*act, *customer, *contract, sellerFromOrganization(org))
	if err != nil {
		return nil, appError("XML_VALIDATION_FAILED", err.Error(), true, "Исправьте реквизиты или строки акта")
	}
	path, safeName, err := s.documentPath(org.ID, act.Date, "act", input.ID, filename)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return nil, err
	}
	result, err := s.upsertDocumentFile(ctx, org.ID, "act", input.ID, "upd_xml", "application/xml", path, safeName, int64(len(data)))
	if err != nil {
		_ = os.Remove(path)
		return nil, appError("STORAGE_ERROR", "Файл УПД сформирован, но не удалось сохранить запись о нём — повторите запрос", true, "Повторите acts.export_upd_xml")
	}
	return result, nil
}

func (s *Service) ValidateActUPDXML(ctx context.Context, input IDInput) (*ValidationResult, error) {
	org, err := s.CurrentOrganization(ctx)
	if err != nil {
		return nil, err
	}
	act, err := s.db.GetActWithServices(ctx, input.ID)
	if err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните ID")
	}
	customer, err := s.db.GetCustomerByID(ctx, act.CustomerID)
	if err != nil {
		return nil, err
	}
	contract, err := s.db.GetContractByID(ctx, act.ContractID)
	if err != nil {
		return nil, err
	}
	data, _, err := updxml.BuildActUPDXML(*act, *customer, *contract, sellerFromOrganization(org))
	if err != nil {
		return nil, appError("XML_VALIDATION_FAILED", err.Error(), true, "Исправьте реквизиты или строки акта")
	}
	// Best-effort reference to a previously stored file, if any — validation never writes.
	file, _ := s.GetFile(ctx, RenderFileInput{DocumentType: "act", DocumentID: input.ID})
	if err := xml.Unmarshal(data, new(any)); err != nil {
		return &ValidationResult{Status: "failed", XMLParse: err.Error(), XSDValidation: "not_run"}, nil
	}
	text := string(data)
	checks := []string{"xml_parse_ok"}
	warnings := []string{"XSD validation is available in Go tests when xmllint is installed; runtime validation currently performs parser and business checks."}
	required := []string{`ВерсФорм="5.03"`, `КНД="1115131"`, `Функция="ДОП"`, `НалСт="без НДС"`, `<БезНДС>без НДС</БезНДС>`, `СодОпер="Оказание услуг"`}
	for _, item := range required {
		if !strings.Contains(text, item) {
			return &ValidationResult{Status: "failed", XMLParse: "ok", XSDValidation: "not_run", BusinessChecks: checks, Warnings: append(warnings, "Не найдено: "+item), File: file}, nil
		}
		checks = append(checks, "contains "+item)
	}
	for _, forbidden := range []string{`Функция="СЧФДОП"`, `НалСт="0%"`, `НалСт="5%"`, `НалСт="7%"`, `НалСт="10%"`, `НалСт="20%"`} {
		if strings.Contains(text, forbidden) {
			return &ValidationResult{Status: "failed", XMLParse: "ok", XSDValidation: "not_run", BusinessChecks: checks, Warnings: append(warnings, "Запрещённый фрагмент: "+forbidden), File: file}, nil
		}
	}
	return &ValidationResult{Status: "passed", XMLParse: "ok", XSDValidation: "not_run", BusinessChecks: checks, Warnings: warnings, File: file}, nil
}

func (s *Service) GetFile(ctx context.Context, input RenderFileInput) (*FileResult, error) {
	docType := normalizeDocumentType(input.DocumentType)
	if docType == "" {
		return nil, appError("VALIDATION_ERROR", "document_type должен быть invoice или act", true, "Исправьте тип документа")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, document_type, document_id::text, file_kind, mime_type, file_name, storage_path, size_bytes
		FROM document_files
		WHERE document_type=$1 AND document_id=$2::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`, docType, input.DocumentID)
	var f FileResult
	if err := row.Scan(&f.ID, &f.DocumentType, &f.DocumentID, &f.FileKind, &f.MimeType, &f.FileName, &f.StoragePath, &f.SizeBytes); err != nil {
		return nil, appError("DOCUMENT_NOT_FOUND", "Файл документа не найден", true, "Сначала вызовите render_pdf или export_upd_xml")
	}
	return &f, nil
}

func (s *Service) Resource(ctx context.Context, uri string) (map[string]any, error) {
	switch {
	case uri == "accounting://organization/current":
		org, err := s.CurrentOrganization(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"data": org}, nil
	case uri == "accounting://reports/unpaid-invoices":
		return s.ListUnpaidInvoices(ctx, SearchInput{Limit: 100})
	case strings.HasPrefix(uri, "accounting://counterparties/"):
		id := strings.TrimPrefix(uri, "accounting://counterparties/")
		return s.GetCounterparty(ctx, IDInput{ID: id})
	case strings.HasPrefix(uri, "accounting://contracts/"):
		id := strings.TrimPrefix(uri, "accounting://contracts/")
		return s.GetContract(ctx, IDInput{ID: id})
	case strings.HasPrefix(uri, "accounting://invoices/"):
		id := strings.TrimPrefix(uri, "accounting://invoices/")
		return s.GetInvoice(ctx, IDInput{ID: id})
	case strings.HasPrefix(uri, "accounting://acts/"):
		id := strings.TrimPrefix(uri, "accounting://acts/")
		return s.GetAct(ctx, IDInput{ID: id})
	case strings.HasPrefix(uri, "accounting://documents/") && strings.HasSuffix(uri, "/files"):
		id := strings.TrimSuffix(strings.TrimPrefix(uri, "accounting://documents/"), "/files")
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, document_type, document_id::text, file_kind, mime_type, file_name, storage_path, size_bytes
			FROM document_files
			WHERE document_id=$1::uuid
			ORDER BY created_at DESC
		`, id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var files []FileResult
		for rows.Next() {
			var f FileResult
			if err := rows.Scan(&f.ID, &f.DocumentType, &f.DocumentID, &f.FileKind, &f.MimeType, &f.FileName, &f.StoragePath, &f.SizeBytes); err != nil {
				return nil, err
			}
			files = append(files, f)
		}
		return map[string]any{"data": files}, rows.Err()
	default:
		return nil, appError("DOCUMENT_NOT_FOUND", "Ресурс не найден", true, "Проверьте URI ресурса")
	}
}

func (s *Service) PrepareAction(ctx context.Context, action string, input IDInput, summary string) (*ConfirmationResponse, error) {
	return s.createConfirmation(ctx, action, UserFromContext(ctx), map[string]any{"id": input.ID}, map[string]any{"id": input.ID}, nil, summary)
}

type dbLine struct {
	ServiceID string
	Title     string
	Unit      string
	Price     string
	Qty       string
	Amount    string
}

func (s *Service) prepareDocument(ctx context.Context, counterpartyID, counterpartyQuery, contractID, contractNumber string, inputLines []MoneyLineInput) (*models.Customer, *models.Contract, []PreviewLine, int64, []string, error) {
	customer, err := s.resolveCounterparty(ctx, counterpartyID, counterpartyQuery)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}
	contract, err := s.resolveContract(ctx, customer.ID, contractID, contractNumber)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}
	lines, totalCents, warnings, err := s.previewLines(ctx, inputLines)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}
	return customer, contract, lines, totalCents, warnings, nil
}

func (s *Service) resolveCounterparty(ctx context.Context, id, query string) (*models.Customer, error) {
	if strings.TrimSpace(id) != "" {
		return s.db.GetCustomerByID(ctx, id)
	}
	result, err := s.SearchCounterparties(ctx, SearchInput{Query: query, Limit: 10})
	if err != nil {
		return nil, err
	}
	items, _ := result["data"].([]models.Customer)
	if len(items) == 0 {
		return nil, appError("COUNTERPARTY_NOT_FOUND", "Контрагент не найден", true, "Уточните ИНН или создайте контрагента")
	}
	if len(items) > 1 {
		return nil, &AccountingError{Code: "COUNTERPARTY_DUPLICATE", Message: "Найдено несколько контрагентов", Details: map[string]any{"matches": items}, Recoverable: true, SuggestedAction: "Передайте точный counterparty_id"}
	}
	return &items[0], nil
}

func (s *Service) resolveContract(ctx context.Context, customerID, contractID, contractNumber string) (*models.Contract, error) {
	if contractID != "" {
		return s.db.GetContractByID(ctx, contractID)
	}
	if contractNumber != "" {
		return s.db.GetContractByCustomerAndNumber(ctx, customerID, contractNumber)
	}
	contracts, _, err := s.db.GetContracts(ctx, customerID, 1, 20)
	if err != nil {
		return nil, err
	}
	active := make([]models.Contract, 0)
	for _, c := range contracts {
		if c.Status == "active" {
			active = append(active, c)
		}
	}
	if len(active) == 0 {
		return nil, appError("CONTRACT_NOT_FOUND", "У контрагента нет действующих договоров", true, "Создайте договор или передайте contract_id")
	}
	if len(active) > 1 {
		return nil, &AccountingError{Code: "MULTIPLE_CONTRACTS_FOUND", Message: "Найдено несколько действующих договоров", Details: map[string]any{"contracts": active}, Recoverable: true, SuggestedAction: "Передайте contract_id"}
	}
	return &active[0], nil
}

func (s *Service) previewLines(ctx context.Context, inputs []MoneyLineInput) ([]PreviewLine, int64, []string, error) {
	if len(inputs) == 0 {
		return nil, 0, nil, appError("VALIDATION_ERROR", "Нужна хотя бы одна строка документа", true, "Передайте lines")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	defer tx.Rollback()
	lines, total, err := s.buildLinesTx(ctx, tx, inputs)
	if err != nil {
		return nil, 0, nil, err
	}
	preview := make([]PreviewLine, 0, len(lines))
	for _, line := range lines {
		amountCents, _ := parseMoneyCents(line.Amount)
		preview = append(preview, PreviewLine{ServiceID: line.ServiceID, Title: line.Title, Unit: line.Unit, Price: line.Price, Qty: line.Qty, Amount: line.Amount, VAT: "без НДС", AmountCents: amountCents})
	}
	return preview, total, nil, nil
}

func (s *Service) buildLinesTx(ctx context.Context, tx *sql.Tx, inputs []MoneyLineInput) ([]dbLine, int64, error) {
	lines := make([]dbLine, 0, len(inputs))
	var total int64
	for _, input := range inputs {
		unit := strings.TrimSpace(input.Unit)
		if unit == "" {
			unit = "шт"
		}
		qty := strings.TrimSpace(input.Qty)
		if qty == "" {
			qty = "1"
		}
		title := strings.TrimSpace(input.Title)
		price := strings.TrimSpace(input.Price)
		serviceID := strings.TrimSpace(input.ServiceID)
		if serviceID != "" {
			if err := tx.QueryRowContext(ctx, `SELECT name, price::text FROM services WHERE id=$1`, serviceID).Scan(&title, &price); err != nil {
				return nil, 0, appError("VALIDATION_ERROR", "Услуга не найдена", true, "Проверьте service_id")
			}
		}
		if title == "" || price == "" {
			return nil, 0, appError("VALIDATION_ERROR", "У строки должны быть title и price", true, "Исправьте строки документа")
		}
		priceCents, err := parseMoneyCents(price)
		if err != nil {
			return nil, 0, appError("VALIDATION_ERROR", "Некорректная цена: "+price, true, "Передайте цену в рублях")
		}
		qtyMilli, err := parseQtyMilli(qty)
		if err != nil || qtyMilli <= 0 {
			return nil, 0, appError("VALIDATION_ERROR", "Некорректное количество: "+qty, true, "Передайте положительное количество")
		}
		amountCents := (priceCents*qtyMilli + 500) / 1000
		total += amountCents
		lines = append(lines, dbLine{ServiceID: serviceID, Title: title, Unit: unit, Price: decimalFromCents(priceCents), Qty: decimalFromMilli(qtyMilli), Amount: decimalFromCents(amountCents)})
	}
	return lines, total, nil
}

func (s *Service) createConfirmation(ctx context.Context, action, userID string, payload map[string]any, preview map[string]any, warnings []string, summary string) (*ConfirmationResponse, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	payloadBytes, err := canonicalJSONBytes(payload)
	if err != nil {
		return nil, err
	}
	hash := hashBytes(payloadBytes)
	expiresAt := time.Now().Add(s.config.TokenTTL)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO accounting_confirmation_tokens (token, user_id, action, payload_hash, payload, expires_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
	`, token, userID, action, hash, string(payloadBytes), expiresAt); err != nil {
		return nil, err
	}
	return &ConfirmationResponse{Status: "confirmation_required", Summary: summary, Preview: preview, Warnings: warnings, ConfirmationToken: token, ExpiresAt: expiresAt}, nil
}

func (s *Service) commitWithConfirmation(ctx context.Context, action string, input CommitInput, target any, fn func(*sql.Tx) (map[string]any, error)) (map[string]any, error) {
	if !tokenRe.MatchString(input.ConfirmationToken) || strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, appError("VALIDATION_ERROR", "Нужны confirmation_token и idempotency_key", true, "Передайте токен подтверждения и ключ идемпотентности")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var storedAction, userID, payloadHash string
	var payloadRaw []byte
	var expiresAt time.Time
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT action, user_id, payload_hash, payload, expires_at, used_at
		FROM accounting_confirmation_tokens
		WHERE token = $1
		FOR UPDATE
	`, input.ConfirmationToken).Scan(&storedAction, &userID, &payloadHash, &payloadRaw, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return nil, appError("CONFIRMATION_EXPIRED", "Токен подтверждения не найден", true, "Повторите prepare")
	}
	if err != nil {
		return nil, err
	}
	if storedAction != action {
		return nil, appError("CONFIRMATION_MISMATCH", "Токен выдан для другого действия", true, "Повторите prepare")
	}
	if userID != UserFromContext(ctx) {
		return nil, appError("FORBIDDEN", "Токен выдан другому пользователю", true, "Повторите prepare от текущего пользователя")
	}
	if usedAt.Valid {
		return nil, appError("CONFIRMATION_EXPIRED", "Токен уже использован", true, "Повторите prepare")
	}
	if time.Now().After(expiresAt) {
		return nil, appError("CONFIRMATION_EXPIRED", "Токен подтверждения истёк", true, "Повторите prepare")
	}
	storedPayloadBytes, err := canonicalJSONBytes(payloadRaw)
	if err != nil {
		return nil, err
	}
	if hashBytes(storedPayloadBytes) != payloadHash {
		return nil, appError("CONFIRMATION_MISMATCH", "Данные подтверждения изменились", true, "Повторите prepare")
	}
	if err := json.Unmarshal(payloadRaw, target); err != nil {
		return nil, err
	}
	existing, err := s.getIdempotentResultTx(ctx, tx, action, input.IdempotencyKey, payloadHash)
	if err != nil || existing != nil {
		return existing, err
	}
	result, err := fn(tx)
	if err != nil {
		_ = s.writeAuditTx(ctx, tx, action, input.IdempotencyKey, "", "", "error", codeOf(err))
		return nil, err
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO accounting_idempotency_keys (action, idempotency_key, payload_hash, result)
		VALUES ($1, $2, $3, $4::jsonb)
	`, action, input.IdempotencyKey, payloadHash, string(resultBytes)); err != nil {
		if isUniqueViolation(err) {
			// A concurrent commit under a different confirmation_token raced us to the
			// same idempotency_key and won — our writes above are rolled back with this
			// transaction, so nothing is duplicated. Fetch the winner's result instead
			// of surfacing a raw Postgres constraint error.
			_ = tx.Rollback()
			existing, lookupErr := s.getIdempotentResultDB(ctx, action, input.IdempotencyKey, payloadHash)
			if lookupErr == nil && existing != nil {
				return existing, nil
			}
			return nil, appError("IDEMPOTENCY_CONFLICT", "Операция с этим idempotency_key уже обрабатывается или обработана параллельным запросом", true, "Повторите запрос с тем же idempotency_key через несколько секунд")
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounting_confirmation_tokens SET used_at=CURRENT_TIMESTAMP WHERE token=$1`, input.ConfirmationToken); err != nil {
		return nil, err
	}
	_ = s.writeAuditTx(ctx, tx, action, input.IdempotencyKey, documentTypeFromAction(action), documentIDFromResult(result), "success", "")
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// getIdempotentResultDB is the getIdempotentResultTx equivalent for use after the
// calling transaction has already been rolled back (e.g. lost the idempotency-key
// race), when a *sql.Tx can no longer be queried.
func (s *Service) getIdempotentResultDB(ctx context.Context, action, key, payloadHash string) (map[string]any, error) {
	var storedHash string
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT payload_hash, result
		FROM accounting_idempotency_keys
		WHERE action=$1 AND idempotency_key=$2
	`, action, key).Scan(&storedHash, &raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if storedHash != payloadHash {
		return nil, appError("CONFIRMATION_MISMATCH", "idempotency_key уже использован с другими данными", true, "Передайте новый idempotency_key")
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result["idempotent_replay"] = true
	return result, nil
}

func (s *Service) getIdempotentResultTx(ctx context.Context, tx *sql.Tx, action, key, payloadHash string) (map[string]any, error) {
	var storedHash string
	var raw []byte
	err := tx.QueryRowContext(ctx, `
		SELECT payload_hash, result
		FROM accounting_idempotency_keys
		WHERE action=$1 AND idempotency_key=$2
	`, action, key).Scan(&storedHash, &raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if storedHash != payloadHash {
		return nil, appError("CONFIRMATION_MISMATCH", "idempotency_key уже использован с другими данными", true, "Передайте новый idempotency_key")
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result["idempotent_replay"] = true
	return result, nil
}

func (s *Service) currentOrganizationTx(ctx context.Context, tx *sql.Tx) (*Organization, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, full_name, short_name, inn, vat_mode, timezone FROM organizations WHERE active=true ORDER BY created_at LIMIT 1`)
	var org Organization
	if err := row.Scan(&org.ID, &org.FullName, &org.ShortName, &org.INN, &org.VATMode, &org.Timezone); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError("ORGANIZATION_NOT_CONFIGURED", "В системе не настроена активная организация-продавец", false, "Обратитесь к администратору для настройки организации")
		}
		return nil, err
	}
	return &org, nil
}

func (s *Service) nextNumberTx(ctx context.Context, tx *sql.Tx, orgID, docType string) (int64, error) {
	var existingMax int64
	table := "invoices"
	if docType == "act" {
		table = "acts"
	} else if docType == "contract" {
		table = "contracts"
	}
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(number::bigint), $1) FROM %s WHERE number ~ '^[0-9]+$'`, table), startNumber(docType)).Scan(&existingMax); err != nil {
		return 0, err
	}
	var next int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO accounting_number_sequences (organization_id, document_type, period_year, last_number)
		VALUES ($1::uuid, $2, 0, $3)
		ON CONFLICT (organization_id, document_type, period_year)
		DO UPDATE SET last_number = GREATEST(accounting_number_sequences.last_number + 1, EXCLUDED.last_number),
		              updated_at = CURRENT_TIMESTAMP
		RETURNING last_number
	`, orgID, docType, existingMax+1).Scan(&next)
	return next, err
}

func (s *Service) bumpNumberSequenceTx(ctx context.Context, tx *sql.Tx, docType string, number int64) error {
	org, err := s.currentOrganizationTx(ctx, tx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO accounting_number_sequences (organization_id, document_type, period_year, last_number)
		VALUES ($1::uuid, $2, 0, $3)
		ON CONFLICT (organization_id, document_type, period_year)
		DO UPDATE SET last_number = GREATEST(accounting_number_sequences.last_number, EXCLUDED.last_number),
		              updated_at = CURRENT_TIMESTAMP
	`, org.ID, docType, number)
	return err
}

func startNumber(docType string) int64 {
	if docType == "contract" {
		return 699
	}
	return 2999
}

// normalizeContractTopic delegates to the shared contracttopics package (also
// used by the REST handlers) so both channels validate and map contract
// topics identically.
func normalizeContractTopic(topic string) (string, error) {
	normalized, err := contracttopics.Normalize(topic)
	if err != nil {
		return "", appError("VALIDATION_ERROR", err.Error(), true, "Используйте одну из валидных тем: "+strings.Join(allowedContractTopics(), ", "))
	}
	return normalized, nil
}

func allowedContractTopics() []string {
	return contracttopics.Allowed()
}

func (s *Service) findCounterpartyDuplicates(ctx context.Context, inn, kpp string) ([]models.Customer, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return s.findCounterpartyDuplicatesTx(ctx, tx, inn, kpp)
}

func (s *Service) findCounterpartyDuplicatesTx(ctx context.Context, tx *sql.Tx, inn, kpp string) ([]models.Customer, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, fullname, address, inn, COALESCE(kpp, ''),
		       COALESCE(edo_id_tensor, ''), COALESCE(edo_id_kontur, ''), COALESCE(okpo, ''),
		       COALESCE(phone, ''), COALESCE(email, ''), COALESCE(contact_person, ''),
		       COALESCE(contact_position, ''), COALESCE(comment, ''), COALESCE(status, 'active'),
		       created_at, updated_at
		FROM customers
		WHERE status <> 'archived'
		  AND inn = $1
		  AND ($2 = '' OR COALESCE(kpp, '') = $2)
		LIMIT 10
	`, inn, kpp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []models.Customer
	for rows.Next() {
		var c models.Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Fullname, &c.Address, &c.INN, &c.KPP, &c.EDOIDTensor, &c.EDOIDKontur, &c.OKPO, &c.Phone, &c.Email, &c.ContactPerson, &c.ContactPosition, &c.Comment, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		matches = append(matches, c)
	}
	return matches, rows.Err()
}

func (s *Service) findInvoiceDuplicates(ctx context.Context, contractID, date string, totalCents int64) ([]models.Invoice, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
		FROM invoices
		WHERE contract_id=$1 AND date=$2 AND total_amount=$3::numeric AND archived=false
		LIMIT 10
	`, contractID, date, decimalFromCents(totalCents))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invoices []models.Invoice
	for rows.Next() {
		var invoice models.Invoice
		if err := rows.Scan(&invoice.ID, &invoice.ContractID, &invoice.CustomerID, &invoice.Number, &invoice.Date, &invoice.Status, &invoice.TotalAmount, &invoice.Archived, &invoice.ContractNumber, &invoice.CreatedAt, &invoice.UpdatedAt); err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, rows.Err()
}

func (s *Service) upsertDocumentFile(ctx context.Context, orgID, docType, docID, kind, mimeType, path, fileName string, size int64) (*FileResult, error) {
	var result FileResult
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO document_files (organization_id, document_type, document_id, file_kind, mime_type, storage_path, file_name, size_bytes)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8)
		ON CONFLICT (document_type, document_id, file_kind)
		DO UPDATE SET mime_type=EXCLUDED.mime_type, storage_path=EXCLUDED.storage_path, file_name=EXCLUDED.file_name, size_bytes=EXCLUDED.size_bytes, created_at=CURRENT_TIMESTAMP
		RETURNING id, document_type, document_id::text, file_kind, mime_type, file_name, storage_path, size_bytes
	`, orgID, docType, docID, kind, mimeType, path, fileName, size).
		Scan(&result.ID, &result.DocumentType, &result.DocumentID, &result.FileKind, &result.MimeType, &result.FileName, &result.StoragePath, &result.SizeBytes)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) documentPath(orgID, ruDate, docType, docID, fileName string) (string, string, error) {
	t, err := time.Parse("02.01.2006", ruDate)
	if err != nil {
		t = time.Now()
	}
	safeName := sanitizeFileName(fileName)
	rel := filepath.Join(orgID, t.Format("2006"), t.Format("01"), docType+"s", docID, safeName)
	root, err := filepath.Abs(s.config.DocumentStoragePath)
	if err != nil {
		return "", "", err
	}
	full := filepath.Join(root, rel)
	clean := filepath.Clean(full)
	if !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", "", appError("STORAGE_ERROR", "Некорректный путь файла", false, "")
	}
	return clean, safeName, nil
}

func (s *Service) writeAuditTx(ctx context.Context, tx *sql.Tx, action, idempotencyKey, docType, docID, result, errorCode string) error {
	var uuidDoc any
	if docID != "" {
		uuidDoc = docID
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor, mcp_client, tool, document_type, document_id, action, result, error_code, idempotency_key)
		VALUES ($1, 'hermes', $2, NULLIF($3, ''), $4::uuid, $2, $5, NULLIF($6, ''), NULLIF($7, ''))
	`, UserFromContext(ctx), action, docType, uuidDoc, result, errorCode, idempotencyKey)
	return err
}

func updateInvoiceStatusTx(ctx context.Context, tx *sql.Tx, id, status string) (*models.Invoice, error) {
	var invoice models.Invoice
	err := tx.QueryRowContext(ctx, `
		UPDATE invoices SET status=$2 WHERE id=$1
		RETURNING id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
	`, id, status).Scan(&invoice.ID, &invoice.ContractID, &invoice.CustomerID, &invoice.Number, &invoice.Date, &invoice.Status, &invoice.TotalAmount, &invoice.Archived, &invoice.ContractNumber, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните ID")
	}
	return &invoice, err
}

func updateActStatusTx(ctx context.Context, tx *sql.Tx, id, status string) (*models.Act, error) {
	var act models.Act
	err := tx.QueryRowContext(ctx, `
		UPDATE acts SET status=$2 WHERE id=$1
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`, id, status).Scan(&act.ID, &act.ContractID, &act.Number, &act.Date, &act.Status, &act.TotalAmount, &act.Archived, &act.CreatedAt, &act.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните ID")
	}
	if err != nil {
		return nil, err
	}
	_ = tx.QueryRowContext(ctx, `SELECT customer_id, number FROM contracts WHERE id=$1`, act.ContractID).Scan(&act.CustomerID, &act.ContractNumber)
	return &act, nil
}

func (s *Service) documentNumberPreview(ctx context.Context, docType, docID, number string) (any, any, error) {
	if docType == "invoice" {
		invoice, err := s.db.GetInvoiceByID(ctx, docID)
		if err != nil {
			return nil, nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните document_id")
		}
		duplicate, err := s.db.CheckInvoiceNumberExists(ctx, invoice.ContractID, number, invoice.ID)
		if err != nil {
			return nil, nil, err
		}
		if duplicate {
			return invoice, map[string]any{"contract_id": invoice.ContractID, "number": number}, nil
		}
		return invoice, nil, nil
	}

	act, err := s.db.GetActByID(ctx, docID)
	if err != nil {
		return nil, nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните document_id")
	}
	duplicate, err := s.db.CheckActNumberExists(ctx, act.ContractID, number, act.ID)
	if err != nil {
		return nil, nil, err
	}
	if duplicate {
		return act, map[string]any{"contract_id": act.ContractID, "number": number}, nil
	}
	return act, nil, nil
}

func updateInvoiceNumberTx(ctx context.Context, tx *sql.Tx, id, number string) (*models.Invoice, error) {
	var contractID string
	if err := tx.QueryRowContext(ctx, `SELECT contract_id FROM invoices WHERE id=$1 FOR UPDATE`, id).Scan(&contractID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните document_id")
		}
		return nil, err
	}
	if exists, err := invoiceNumberExistsTx(ctx, tx, contractID, number, id); err != nil {
		return nil, err
	} else if exists {
		return nil, appError("DOCUMENT_NUMBER_CONFLICT", "Номер счёта уже занят в этом договоре", true, "Выберите другой номер")
	}
	var invoice models.Invoice
	err := tx.QueryRowContext(ctx, `
		UPDATE invoices SET number=$2 WHERE id=$1
		RETURNING id, contract_id, customer_id, number, date, status, total_amount, archived, contract_number, created_at, updated_at
	`, id, number).Scan(&invoice.ID, &invoice.ContractID, &invoice.CustomerID, &invoice.Number, &invoice.Date, &invoice.Status, &invoice.TotalAmount, &invoice.Archived, &invoice.ContractNumber, &invoice.CreatedAt, &invoice.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, appError("DOCUMENT_NUMBER_CONFLICT", "Номер счёта уже занят в этом договоре", true, "Выберите другой номер")
	}
	return &invoice, err
}

func updateActNumberTx(ctx context.Context, tx *sql.Tx, id, number string) (*models.Act, error) {
	var contractID string
	if err := tx.QueryRowContext(ctx, `SELECT contract_id FROM acts WHERE id=$1 FOR UPDATE`, id).Scan(&contractID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appError("DOCUMENT_NOT_FOUND", "Акт не найден", true, "Уточните document_id")
		}
		return nil, err
	}
	if exists, err := actNumberExistsTx(ctx, tx, contractID, number, id); err != nil {
		return nil, err
	} else if exists {
		return nil, appError("DOCUMENT_NUMBER_CONFLICT", "Номер акта уже занят в этом договоре", true, "Выберите другой номер")
	}
	var act models.Act
	err := tx.QueryRowContext(ctx, `
		UPDATE acts SET number=$2 WHERE id=$1
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`, id, number).Scan(&act.ID, &act.ContractID, &act.Number, &act.Date, &act.Status, &act.TotalAmount, &act.Archived, &act.CreatedAt, &act.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, appError("DOCUMENT_NUMBER_CONFLICT", "Номер акта уже занят в этом договоре", true, "Выберите другой номер")
	}
	if err != nil {
		return nil, err
	}
	_ = tx.QueryRowContext(ctx, `SELECT customer_id, number FROM contracts WHERE id=$1`, act.ContractID).Scan(&act.CustomerID, &act.ContractNumber)
	return &act, nil
}

func invoiceNumberExistsTx(ctx context.Context, tx *sql.Tx, contractID, number, excludeID string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM invoices
			WHERE contract_id=$1 AND number=$2 AND id<>$3::uuid
		)
	`, contractID, number, excludeID).Scan(&exists)
	return exists, err
}

func actNumberExistsTx(ctx context.Context, tx *sql.Tx, contractID, number, excludeID string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM acts
			WHERE contract_id=$1 AND number=$2 AND id<>$3::uuid
		)
	`, contractID, number, excludeID).Scan(&exists)
	return exists, err
}

func createActFromInvoiceTx(ctx context.Context, tx *sql.Tx, invoiceID, number, date, status string) (*models.Act, error) {
	var contractID, customerID, contractNumber string
	if err := tx.QueryRowContext(ctx, `SELECT contract_id, customer_id, contract_number FROM invoices WHERE id = $1`, invoiceID).Scan(&contractID, &customerID, &contractNumber); err != nil {
		if err == sql.ErrNoRows {
			return nil, appError("DOCUMENT_NOT_FOUND", "Счёт не найден", true, "Уточните invoice_id")
		}
		return nil, err
	}
	var act models.Act
	err := tx.QueryRowContext(ctx, `
		INSERT INTO acts (contract_id, number, date, status, total_amount, archived)
		VALUES ($1, $2, $3, $4, 0, false)
		RETURNING id, contract_id, number, date, status, total_amount, archived, created_at, updated_at
	`, contractID, number, date, defaultString(status, "draft")).
		Scan(&act.ID, &act.ContractID, &act.Number, &act.Date, &act.Status, &act.TotalAmount, &act.Archived, &act.CreatedAt, &act.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO act_lines (act_id, service_id, title_snapshot, unit_snapshot, vat_snapshot, price_snapshot, qty, amount)
		SELECT $1, service_id, title_snapshot, unit_snapshot, 0, price_snapshot, qty, amount
		FROM invoice_lines
		WHERE invoice_id = $2
	`, act.ID, invoiceID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO act_invoices (act_id, invoice_id) VALUES ($1, $2)`, act.ID, invoiceID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE acts SET total_amount=COALESCE((SELECT SUM(amount) FROM act_lines WHERE act_id=$1), 0)
		WHERE id=$1
	`, act.ID); err != nil {
		return nil, err
	}
	act.CustomerID = customerID
	act.ContractNumber = contractNumber
	return &act, nil
}

func parseMoneyCents(value string) (int64, error) {
	clean := strings.ReplaceAll(strings.TrimSpace(value), " ", "")
	clean = strings.ReplaceAll(clean, ",", ".")
	if clean == "" || strings.HasPrefix(clean, "-") {
		return 0, errors.New("invalid money")
	}
	parts := strings.Split(clean, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid money")
	}
	rubles, err := strconv.ParseInt(defaultString(parts[0], "0"), 10, 64)
	if err != nil {
		return 0, err
	}
	kopecks := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 2 {
			fraction = fraction[:2]
		}
		for len(fraction) < 2 {
			fraction += "0"
		}
		kopecks, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return rubles*100 + kopecks, nil
}

func parseQtyMilli(value string) (int64, error) {
	clean := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil || f <= 0 {
		return 0, errors.New("invalid qty")
	}
	return int64(math.Round(f * 1000)), nil
}

func decimalFromCents(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func decimalFromMilli(value int64) string {
	return fmt.Sprintf("%d.%03d", value/1000, value%1000)
}

func formatMoney(cents int64) string {
	return strings.Replace(decimalFromCents(cents), ".", ",", 1)
}

func centsFromFloat(value float64) int64 {
	return int64(math.Round(value * 100))
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func canonicalJSONBytes(value any) ([]byte, error) {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case json.RawMessage:
		raw = []byte(v)
	default:
		marshaled, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		raw = marshaled
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

// moscowLocation is a fixed UTC+3 offset rather than IANA's "Europe/Moscow",
// so it never depends on the tzdata database being installed in the runtime
// image (Alpine's base image doesn't ship it) — time.LoadLocation silently
// returns a nil *Location on such a lookup failure, and Time.In(nil) panics.
var moscowLocation = time.FixedZone("MSK", 3*60*60)

func todayRu() string {
	return time.Now().In(moscowLocation).Format("02.01.2006")
}

func boundedLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func paging(input SearchInput) (int, int) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	perPage := input.PerPage
	if perPage <= 0 {
		perPage = boundedLimit(input.Limit, 50)
	}
	return page, perPage
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func inputID(id string) string {
	return strings.TrimSpace(id)
}

func normalizeDocumentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "invoice", "invoices":
		return "invoice"
	case "act", "acts":
		return "act"
	default:
		return ""
	}
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(os.PathSeparator) || name == "" {
		name = "document"
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func minimalPDF(title string, lines []string) []byte {
	var content bytes.Buffer
	content.WriteString("BT\n/F1 12 Tf\n72 760 Td\n")
	for index, line := range append([]string{title}, lines...) {
		if index > 0 {
			content.WriteString("0 -18 Td\n")
		}
		content.WriteString("(" + pdfEscape(line) + ") Tj\n")
	}
	content.WriteString("ET\n")
	stream := content.String()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		out.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, obj))
	}
	xref := out.Len()
	out.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1))
	for i := 1; i < len(offsets); i++ {
		out.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	out.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref))
	return out.Bytes()
}

func pdfEscape(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "(", "\\(")
	text = strings.ReplaceAll(text, ")", "\\)")
	return text
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}

func numericString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func codeOf(err error) string {
	var app *AccountingError
	if errors.As(err, &app) {
		return app.Code
	}
	return "INTERNAL_ERROR"
}

func documentTypeFromAction(action string) string {
	if strings.HasPrefix(action, "invoices.") {
		return "invoice"
	}
	if strings.HasPrefix(action, "acts.") {
		return "act"
	}
	if strings.HasPrefix(action, "contracts.") {
		return "contract"
	}
	return ""
}

func documentIDFromResult(result map[string]any) string {
	data, ok := result["data"].(models.Invoice)
	if ok {
		return data.ID
	}
	if act, ok := result["data"].(models.Act); ok {
		return act.ID
	}
	if contract, ok := result["data"].(models.Contract); ok {
		return contract.ID
	}
	if customer, ok := result["data"].(models.Customer); ok {
		return customer.ID
	}
	return ""
}

func resourceText(uri string, value any) (*url.URL, string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, "", err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return parsed, string(data), nil
}
