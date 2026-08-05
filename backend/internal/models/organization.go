package models

// OrganizationSigner описывает подписанта (директора/ИП) организации-продавца.
type OrganizationSigner struct {
	Position   string `json:"position"`
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
}

// Organization представляет реквизиты собственной организации (продавца) —
// единый источник данных для печатных форм и УПД XML, вместо зашитых констант.
type Organization struct {
	ID              string             `json:"id"`
	Type            string             `json:"type"`
	FullName        string             `json:"full_name"`
	ShortName       string             `json:"short_name"`
	LastName        string             `json:"last_name,omitempty"`
	FirstName       string             `json:"first_name,omitempty"`
	MiddleName      string             `json:"middle_name,omitempty"`
	INN             string             `json:"inn"`
	KPP             string             `json:"kpp,omitempty"`
	OGRN            string             `json:"ogrn,omitempty"`
	LegalAddress    string             `json:"legal_address,omitempty"`
	PostalAddress   string             `json:"postal_address,omitempty"`
	Phone           string             `json:"phone,omitempty"`
	Email           string             `json:"email,omitempty"`
	BankAccount     string             `json:"bank_account,omitempty"`
	BankName        string             `json:"bank_name,omitempty"`
	BankBIK         string             `json:"bank_bik,omitempty"`
	BankCorrAccount string             `json:"bank_corr_account,omitempty"`
	Signer          OrganizationSigner `json:"signer"`
}

// OrganizationResponse представляет ответ с текущей организацией.
type OrganizationResponse struct {
	Data Organization `json:"data"`
}
