package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"invoices-backend/internal/models"
)

// GetActiveOrganization возвращает текущую активную организацию-продавца.
func (db *DB) GetActiveOrganization(ctx context.Context) (*models.Organization, error) {
	query := `
		SELECT id, org_type, full_name, short_name,
		       COALESCE(last_name, ''), COALESCE(first_name, ''), COALESCE(middle_name, ''),
		       inn, COALESCE(kpp, ''), COALESCE(ogrn, ''),
		       COALESCE(legal_address, ''), COALESCE(postal_address, ''),
		       COALESCE(phone, ''), COALESCE(email, ''),
		       COALESCE(bank_account, ''), COALESCE(bank_name, ''),
		       COALESCE(bank_bik, ''), COALESCE(bank_corr_account, ''),
		       signer
		FROM organizations
		WHERE active = true
		ORDER BY created_at
		LIMIT 1
	`
	var org models.Organization
	var signerRaw []byte
	err := db.QueryRowContext(ctx, query).Scan(
		&org.ID, &org.Type, &org.FullName, &org.ShortName,
		&org.LastName, &org.FirstName, &org.MiddleName,
		&org.INN, &org.KPP, &org.OGRN,
		&org.LegalAddress, &org.PostalAddress,
		&org.Phone, &org.Email,
		&org.BankAccount, &org.BankName, &org.BankBIK, &org.BankCorrAccount,
		&signerRaw,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("organization not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	if len(signerRaw) > 0 {
		_ = json.Unmarshal(signerRaw, &org.Signer)
	}
	if org.Signer.LastName == "" {
		org.Signer.LastName = org.LastName
	}
	if org.Signer.FirstName == "" {
		org.Signer.FirstName = org.FirstName
	}
	if org.Signer.MiddleName == "" {
		org.Signer.MiddleName = org.MiddleName
	}
	return &org, nil
}
