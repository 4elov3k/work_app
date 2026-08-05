package updxml

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"invoices-backend/internal/models"
)

const (
	actFilePrefix        = "ON_NSCHFDOPPR"
	defaultSellerEDOID   = "2BEb25cae8e664f11e38742005056917125"
	sellerFullName       = "Индивидуальный предприниматель Мыленкова Любовь Валерьевна"
	sellerINN            = "526220116209"
	sellerOGRNIP         = "312526227100047"
	sellerAddress        = "603136, г. Нижний Новгород ул, Маршала Рокоссовского, д. 2к1, кв 135"
	sellerPhone          = "8-905-864445"
	sellerBankName       = `ООО "Банк Точка"`
	sellerBIK            = "044525104"
	sellerAccount        = "40802810164270001108"
	sellerCorrAcct       = "30101810445745251004"
	sellerPosition       = "Индивидуальный предприниматель"
	sellerLastName       = "Мыленкова"
	sellerFirstName      = "Любовь"
	sellerMiddleName     = "Валерьевна"
	actTransferOperation = "Оказание услуг"
)

type DocumentType string
type VATMode string

const (
	DocumentTypeAct DocumentType = "act"
	VATModeNone     VATMode      = "none"
	VATModeIncluded VATMode      = "included"
)

type ActUPDOptions struct {
	DocumentType DocumentType
	VATMode      VATMode
	IssueInvoice bool
	SellerEDOID  string
}

var sellerActUPDOptions = ActUPDOptions{
	DocumentType: DocumentTypeAct,
	VATMode:      VATModeNone,
	IssueInvoice: false,
}

// BuildActUPDXML builds a formalized UPD XML title for Saby/Tensor import.
func BuildActUPDXML(act models.ActWithServices, customer models.Customer, contract models.Contract) ([]byte, string, error) {
	return BuildActUPDXMLWithOptions(act, customer, contract, sellerActUPDOptions)
}

func BuildActUPDXMLWithSellerEDOID(act models.ActWithServices, customer models.Customer, contract models.Contract, sellerEDOID string) ([]byte, string, error) {
	options := sellerActUPDOptions
	options.SellerEDOID = sellerEDOID
	return BuildActUPDXMLWithOptions(act, customer, contract, options)
}

func BuildActUPDXMLWithOptions(act models.ActWithServices, customer models.Customer, contract models.Contract, options ActUPDOptions) ([]byte, string, error) {
	return buildActUPDXMLAt(act, customer, contract, options, moscowNow())
}

func buildActUPDXMLAt(act models.ActWithServices, customer models.Customer, contract models.Contract, options ActUPDOptions, formedAt time.Time) ([]byte, string, error) {
	if strings.TrimSpace(act.Number) == "" {
		return nil, "", fmt.Errorf("act number is required")
	}
	if strings.TrimSpace(act.Date) == "" {
		return nil, "", fmt.Errorf("act date is required")
	}
	if err := validateCustomer(customer); err != nil {
		return nil, "", err
	}
	if err := validateSeller(); err != nil {
		return nil, "", err
	}
	if err := validateActUPDOptions(options); err != nil {
		return nil, "", err
	}
	if err := validateContract(contract); err != nil {
		return nil, "", err
	}
	if len(act.Services) == 0 {
		return nil, "", fmt.Errorf("act has no service lines")
	}
	if err := validateServicesForEDO(act.Services, options); err != nil {
		return nil, "", err
	}
	if err := validateActTotal(act); err != nil {
		return nil, "", err
	}
	if err := validateActEDOParticipants(customer, options); err != nil {
		return nil, "", err
	}
	docDate, err := parseRuDate(act.Date)
	if err != nil {
		return nil, "", err
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	enc := xml.NewEncoder(&b)
	enc.Indent("", "  ")

	fileID, err := actFileID(customer, options, formedAt)
	if err != nil {
		return nil, "", err
	}
	root := xml.StartElement{
		Name: xml.Name{Local: "Файл"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "ИдФайл"}, Value: fileID},
			{Name: xml.Name{Local: "ВерсПрог"}, Value: "work_app"},
			{Name: xml.Name{Local: "ВерсФорм"}, Value: "5.03"},
		},
	}
	if err := enc.EncodeToken(root); err != nil {
		return nil, "", err
	}
	if err := writeDocument(enc, act, customer, contract, options, docDate, formedAt); err != nil {
		return nil, "", err
	}
	if err := enc.EncodeToken(root.End()); err != nil {
		return nil, "", err
	}
	if err := enc.Flush(); err != nil {
		return nil, "", err
	}
	return b.Bytes(), xmlFilename(fileID), nil
}

func xmlFilename(fileID string) string {
	return fileID + ".xml"
}

func writeDocument(enc *xml.Encoder, act models.ActWithServices, customer models.Customer, contract models.Contract, options ActUPDOptions, docDate, formedAt time.Time) error {
	doc := xml.StartElement{
		Name: xml.Name{Local: "Документ"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "КНД"}, Value: "1115131"},
			{Name: xml.Name{Local: "Функция"}, Value: documentFunction(options)},
			{Name: xml.Name{Local: "ДатаИнфПр"}, Value: formedAt.Format("02.01.2006")},
			{Name: xml.Name{Local: "ВремИнфПр"}, Value: formedAt.Format("15.04.05")},
			{Name: xml.Name{Local: "НаимЭконСубСост"}, Value: sellerFullName},
			{Name: xml.Name{Local: "ПоФактХЖ"}, Value: "Документ об отгрузке товаров (выполнении работ), передаче имущественных прав (документ об оказании услуг)"},
			{Name: xml.Name{Local: "НаимДокОпр"}, Value: "Универсальный передаточный документ"},
		},
	}
	if err := enc.EncodeToken(doc); err != nil {
		return err
	}
	if err := writeInvoiceInfo(enc, act, customer, docDate); err != nil {
		return err
	}
	if err := writeTable(enc, act.Services, options); err != nil {
		return err
	}
	if err := writeTransferInfo(enc, contract, docDate); err != nil {
		return err
	}
	if err := writeSigner(enc); err != nil {
		return err
	}
	return enc.EncodeToken(doc.End())
}

func writeInvoiceInfo(enc *xml.Encoder, act models.ActWithServices, customer models.Customer, docDate time.Time) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "СвСчФакт"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "НомерДок"}, Value: strings.TrimSpace(act.Number)},
			{Name: xml.Name{Local: "ДатаДок"}, Value: docDate.Format("02.01.2006")},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeSeller(enc); err != nil {
		return err
	}
	if err := writeShipmentDocumentInfo(enc, act, docDate); err != nil {
		return err
	}
	if err := writeBuyer(enc, customer); err != nil {
		return err
	}
	if err := writeCurrency(enc); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeShipmentDocumentInfo(enc *xml.Encoder, act models.ActWithServices, docDate time.Time) error {
	return writeSimpleElement(enc, "ДокПодтвОтгрНом", map[string]string{
		"РеквНаимДок":  "Универсальный передаточный документ",
		"РеквНомерДок": strings.TrimSpace(act.Number),
		"РеквДатаДок":  docDate.Format("02.01.2006"),
	})
}

func writeCurrency(enc *xml.Encoder) error {
	return writeSimpleElement(enc, "ДенИзм", map[string]string{
		"КодОКВ":  "643",
		"НаимОКВ": "Российский рубль",
	})
}

func writeSeller(enc *xml.Encoder) error {
	start := xml.StartElement{Name: xml.Name{Local: "СвПрод"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writePersonID(enc, "ИдСв", sellerINN, sellerOGRNIP, sellerLastName, sellerFirstName, sellerMiddleName); err != nil {
		return err
	}
	if err := writeAddress(enc, sellerAddress); err != nil {
		return err
	}
	if err := writeBankDetails(enc); err != nil {
		return err
	}
	if err := writeContact(enc, sellerPhone); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeBuyer(enc *xml.Encoder, customer models.Customer) error {
	start := xml.StartElement{Name: xml.Name{Local: "СвПокуп"}}
	if strings.TrimSpace(customer.OKPO) != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "ОКПО"}, Value: strings.TrimSpace(customer.OKPO)})
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeBuyerIdentity(enc, customer); err != nil {
		return err
	}
	if err := writeAddress(enc, customer.Address); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeBankDetails(enc *xml.Encoder) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "БанкРекв"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "НомерСчета"}, Value: sellerAccount}},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeSimpleElement(enc, "СвБанк", map[string]string{
		"НаимБанк": sellerBankName,
		"БИК":      sellerBIK,
		"КорСчет":  sellerCorrAcct,
	}); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeContact(enc *xml.Encoder, phone string) error {
	if strings.TrimSpace(phone) == "" {
		return nil
	}
	start := xml.StartElement{Name: xml.Name{Local: "Контакт"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeTextOnlyElement(enc, "Тлф", strings.TrimSpace(phone)); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeBuyerIdentity(enc *xml.Encoder, customer models.Customer) error {
	id := xml.StartElement{Name: xml.Name{Local: "ИдСв"}}
	if err := enc.EncodeToken(id); err != nil {
		return err
	}
	inn := digitsOnly(customer.INN)
	if len(inn) == 12 {
		if err := writePersonID(enc, "", inn, "", "", "", ""); err != nil {
			return err
		}
	} else {
		if err := writeSimpleElement(enc, "СвЮЛУч", map[string]string{
			"НаимОрг": customerName(customer),
			"ИННЮЛ":   inn,
			"КПП":     digitsOnly(customer.KPP),
		}); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(id.End()); err != nil {
		return err
	}
	return nil
}

func writePersonID(enc *xml.Encoder, wrapper, inn, ogrnip, lastName, firstName, middleName string) error {
	var wrapperStart xml.StartElement
	if wrapper != "" {
		wrapperStart = xml.StartElement{Name: xml.Name{Local: wrapper}}
		if err := enc.EncodeToken(wrapperStart); err != nil {
			return err
		}
	}
	attrs := []xml.Attr{{Name: xml.Name{Local: "ИННФЛ"}, Value: inn}}
	if strings.TrimSpace(ogrnip) != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "СвГосРегИП"}, Value: ogrnip})
	}
	ip := xml.StartElement{Name: xml.Name{Local: "СвИП"}, Attr: attrs}
	if err := enc.EncodeToken(ip); err != nil {
		return err
	}
	if strings.TrimSpace(lastName) != "" {
		if err := writeFIO(enc, lastName, firstName, middleName); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(ip.End()); err != nil {
		return err
	}
	if wrapper != "" {
		return enc.EncodeToken(wrapperStart.End())
	}
	return nil
}

func writeFIO(enc *xml.Encoder, lastName, firstName, middleName string) error {
	attrs := map[string]string{
		"Фамилия": lastName,
		"Имя":     firstName,
	}
	if strings.TrimSpace(middleName) != "" {
		attrs["Отчество"] = middleName
	}
	return writeSimpleElement(enc, "ФИО", attrs)
}

func writeAddress(enc *xml.Encoder, address string) error {
	start := xml.StartElement{Name: xml.Name{Local: "Адрес"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeSimpleElement(enc, "АдрИнф", map[string]string{
		"КодСтр":    "643",
		"НаимСтран": "Россия",
		"АдрТекст":  address,
	}); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeTable(enc *xml.Encoder, services []models.Service, options ActUPDOptions) error {
	start := xml.StartElement{Name: xml.Name{Local: "ТаблСчФакт"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	total := 0.0
	totalWithoutVAT := 0.0
	totalVAT := 0.0
	hasVAT := false
	for index, service := range services {
		qty, unitPrice, amount := serviceLineNumbers(service)
		total += amount
		vatRate := effectiveVATRate(service, options)
		if vatRate > 0 {
			hasVAT = true
		}
		withoutVAT, vatAmount := splitVAT(amount, vatRate)
		totalWithoutVAT += withoutVAT
		totalVAT += vatAmount
		unitCode, unitName := unitInfo(service.Unit)
		row := xml.StartElement{
			Name: xml.Name{Local: "СведТов"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "НомСтр"}, Value: fmt.Sprint(index + 1)},
				{Name: xml.Name{Local: "НаимТов"}, Value: strings.TrimSpace(service.Name)},
				{Name: xml.Name{Local: "ОКЕИ_Тов"}, Value: unitCode},
				{Name: xml.Name{Local: "НаимЕдИзм"}, Value: unitName},
				{Name: xml.Name{Local: "КолТов"}, Value: quantityText(qty)},
				{Name: xml.Name{Local: "ЦенаТов"}, Value: moneyText(unitPriceWithoutVAT(unitPrice, vatRate))},
				{Name: xml.Name{Local: "СтТовБезНДС"}, Value: moneyText(withoutVAT)},
				{Name: xml.Name{Local: "НалСт"}, Value: vatRateText(vatRate)},
				{Name: xml.Name{Local: "СтТовУчНал"}, Value: moneyText(amount)},
			},
		}
		if err := enc.EncodeToken(row); err != nil {
			return err
		}
		if err := writeTextElement(enc, "Акциз", "БезАкциз", "без акциза"); err != nil {
			return err
		}
		if err := writeVATAmount(enc, "СумНал", vatAmount, vatRate); err != nil {
			return err
		}
		if vatRate > 0 {
			if err := writeSimpleElement(enc, "ИнфПолФХЖ2", map[string]string{
				"Идентиф": "Цена1С",
				"Значен":  moneyText(unitPrice),
			}); err != nil {
				return err
			}
		}
		if err := enc.EncodeToken(row.End()); err != nil {
			return err
		}
	}
	total = money(total)
	totalWithoutVAT = money(totalWithoutVAT)
	totalVAT = money(totalVAT)
	totalStart := xml.StartElement{
		Name: xml.Name{Local: "ВсегоОпл"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "СтТовБезНДСВсего"}, Value: moneyText(totalWithoutVAT)},
			{Name: xml.Name{Local: "СтТовУчНалВсего"}, Value: moneyText(total)},
		},
	}
	if err := enc.EncodeToken(totalStart); err != nil {
		return err
	}
	if err := writeVATTotal(enc, totalVAT, hasVAT); err != nil {
		return err
	}
	if err := enc.EncodeToken(totalStart.End()); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeTransferInfo(enc *xml.Encoder, contract models.Contract, docDate time.Time) error {
	start := xml.StartElement{Name: xml.Name{Local: "СвПродПер"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}

	transfer := xml.StartElement{
		Name: xml.Name{Local: "СвПер"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "СодОпер"}, Value: actTransferOperation},
			{Name: xml.Name{Local: "ДатаПер"}, Value: docDate.Format("02.01.2006")},
		},
	}
	if err := enc.EncodeToken(transfer); err != nil {
		return err
	}
	if hasContract(contract) {
		contractDate, _ := parseContractDate(contract.StartDate)
		if err := writeSimpleElement(enc, "ОснПер", map[string]string{
			"РеквНаимДок":  "Договор",
			"РеквНомерДок": contractDocumentNumber(contract.Number),
			"РеквДатаДок":  contractDate.Format("02.01.2006"),
		}); err != nil {
			return err
		}
	} else {
		if err := writeTextOnlyElement(enc, "БезДокОснПер", "1"); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(transfer.End()); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeSigner(enc *xml.Encoder) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "Подписант"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "СпосПодтПолном"}, Value: "1"},
			{Name: xml.Name{Local: "Должн"}, Value: sellerPosition},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeFIO(enc, sellerLastName, sellerFirstName, sellerMiddleName); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeSimpleElement(enc *xml.Encoder, name string, attrs map[string]string) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}
	for key, value := range attrs {
		if strings.TrimSpace(value) == "" {
			continue
		}
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: key}, Value: value})
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeTextElement(enc *xml.Encoder, wrapper, child, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: wrapper}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	childStart := xml.StartElement{Name: xml.Name{Local: child}}
	if err := enc.EncodeToken(childStart); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData([]byte(value))); err != nil {
		return err
	}
	if err := enc.EncodeToken(childStart.End()); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeTextOnlyElement(enc *xml.Encoder, name, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData([]byte(value))); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func parseRuDate(value string) (time.Time, error) {
	parsed, err := time.Parse("02.01.2006", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid act date %q: expected DD.MM.YYYY", value)
	}
	return parsed, nil
}

func parseContractDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "02.01.2006"} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid contract date %q: expected YYYY-MM-DD or DD.MM.YYYY", value)
}

func documentFunction(options ActUPDOptions) string {
	if options.IssueInvoice {
		return "СЧФДОП"
	}
	return "ДОП"
}

func effectiveVATRate(service models.Service, options ActUPDOptions) float64 {
	if options.VATMode == VATModeNone {
		return 0
	}
	return service.VAT
}

func validateActUPDOptions(options ActUPDOptions) error {
	if options.DocumentType != DocumentTypeAct {
		return fmt.Errorf("unsupported UPD document type %q", options.DocumentType)
	}
	if options.VATMode != VATModeNone && options.VATMode != VATModeIncluded {
		return fmt.Errorf("unsupported VAT mode %q", options.VATMode)
	}
	if options.VATMode == VATModeNone && options.IssueInvoice {
		return fmt.Errorf("Невозможно сформировать акт: для режима без НДС признак выставления счета-фактуры должен быть выключен")
	}
	return nil
}

func validateSeller() error {
	if len(digitsOnly(sellerINN)) != 12 {
		return fmt.Errorf("seller INN must contain 12 digits for an individual entrepreneur")
	}
	if len(digitsOnly(sellerOGRNIP)) != 15 {
		return fmt.Errorf("seller OGRNIP must contain 15 digits")
	}
	phoneDigits := len(digitsOnly(sellerPhone))
	if phoneDigits < 7 || phoneDigits > 15 {
		return fmt.Errorf("seller phone must contain 7 to 15 digits")
	}
	return nil
}

func hasContract(contract models.Contract) bool {
	return strings.TrimSpace(contract.Number) != "" && strings.TrimSpace(contract.StartDate) != ""
}

func validateContract(contract models.Contract) error {
	hasNumber := strings.TrimSpace(contract.Number) != ""
	hasDate := strings.TrimSpace(contract.StartDate) != ""
	if hasNumber != hasDate {
		return fmt.Errorf("contract number and date must be provided together")
	}
	if !hasNumber {
		return nil
	}
	if _, err := parseContractDate(contract.StartDate); err != nil {
		return err
	}
	if contractDocumentNumber(contract.Number) == "" {
		return fmt.Errorf("contract document number is required")
	}
	return nil
}

func contractDocumentNumber(value string) string {
	trimmed := strings.TrimSpace(value)
	re := regexp.MustCompile(`(?i)№\s*([^\s]+)`)
	if match := re.FindStringSubmatch(trimmed); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return trimmed
}

func validateCustomer(customer models.Customer) error {
	inn := digitsOnly(customer.INN)
	if inn == "" {
		return fmt.Errorf("customer INN is required")
	}
	switch len(inn) {
	case 10:
		kpp := digitsOnly(customer.KPP)
		if kpp == "" {
			return fmt.Errorf("customer KPP is required for organizations")
		}
		if len(kpp) != 9 {
			return fmt.Errorf("customer KPP must contain 9 digits")
		}
	case 12:
	default:
		return fmt.Errorf("customer INN must contain 10 or 12 digits")
	}
	if strings.TrimSpace(customer.Fullname) == "" && strings.TrimSpace(customer.Name) == "" {
		return fmt.Errorf("customer name is required")
	}
	if strings.TrimSpace(customer.Address) == "" {
		return fmt.Errorf("customer address is required")
	}
	return nil
}

func validateActEDOParticipants(customer models.Customer, options ActUPDOptions) error {
	if !validEDOID(sellerParticipantID(options)) {
		return fmt.Errorf("seller EDO participant ID is required")
	}
	if !validEDOID(buyerEDOID(customer)) {
		return fmt.Errorf("customer Tensor EDO participant ID is required")
	}
	return nil
}

func validateServicesForEDO(services []models.Service, options ActUPDOptions) error {
	for index, service := range services {
		lineNumber := index + 1
		if strings.TrimSpace(service.Name) == "" {
			return fmt.Errorf("service line %d name is required", lineNumber)
		}
		if len([]rune(strings.TrimSpace(service.Name))) > 1000 {
			return fmt.Errorf("service line %d name is too long", lineNumber)
		}
		qty, unitPrice, amount := serviceLineNumbers(service)
		if qty <= 0 {
			return fmt.Errorf("service line %d quantity must be positive", lineNumber)
		}
		if unitPrice <= 0 {
			return fmt.Errorf("service line %d price must be positive", lineNumber)
		}
		if amount <= 0 {
			return fmt.Errorf("service line %d amount must be positive", lineNumber)
		}
		if service.VAT < 0 {
			return fmt.Errorf("service line %d VAT rate must not be negative", lineNumber)
		}
		if options.VATMode == VATModeNone && service.VAT != 0 {
			return fmt.Errorf("service line %d VAT rate must be zero in no-VAT mode", lineNumber)
		}
		if money(unitPrice*qty) != amount {
			return fmt.Errorf("service line %d amount must equal price multiplied by quantity", lineNumber)
		}
	}
	return nil
}

func validateActTotal(act models.ActWithServices) error {
	var linesTotal float64
	for _, service := range act.Services {
		_, _, amount := serviceLineNumbers(service)
		linesTotal += amount
	}
	linesTotal = money(linesTotal)
	if act.TotalAmount < 0 {
		return fmt.Errorf("act total amount must not be negative")
	}
	if act.TotalAmount > 0 && money(act.TotalAmount) != linesTotal {
		return fmt.Errorf("act total amount must equal the sum of service lines")
	}
	return nil
}

func serviceLineNumbers(service models.Service) (qty, unitPrice, amount float64) {
	qty = service.Qty
	if qty == 0 {
		qty = 1
	}
	amount = service.Amount
	if amount > 0 {
		amount = money(amount)
		unitPrice = money(amount / qty)
		return qty, unitPrice, amount
	}
	unitPrice = money(service.Price)
	amount = money(unitPrice * qty)
	return qty, unitPrice, amount
}

func money(value float64) float64 {
	return math.Round(value*100) / 100
}

func moneyText(value float64) string {
	return fmt.Sprintf("%.2f", money(value))
}

func quantityText(value float64) string {
	if math.Mod(value, 1) == 0 {
		return fmt.Sprintf("%.0f", value)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}

func unitInfo(unit string) (code, name string) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "усл", "услуга":
		return "876", "усл"
	case "шт", "штука", "штуки":
		return "796", "шт"
	default:
		return "796", strings.TrimSpace(unit)
	}
}

func splitVAT(amount, vatRate float64) (withoutVAT, vatAmount float64) {
	if vatRate <= 0 {
		return money(amount), 0
	}
	withoutVAT = money(amount / (1 + vatRate/100))
	vatAmount = money(amount - withoutVAT)
	return withoutVAT, vatAmount
}

func unitPriceWithoutVAT(unitPrice, vatRate float64) float64 {
	if vatRate <= 0 {
		return money(unitPrice)
	}
	return money(unitPrice / (1 + vatRate/100))
}

func vatRateText(vatRate float64) string {
	if vatRate <= 0 {
		return "без НДС"
	}
	if math.Mod(vatRate, 1) == 0 {
		return fmt.Sprintf("%.0f%%", vatRate)
	}
	return fmt.Sprintf("%.2f%%", vatRate)
}

func writeVATAmount(enc *xml.Encoder, wrapper string, vatAmount, vatRate float64) error {
	if vatRate <= 0 {
		return writeTextElement(enc, wrapper, "БезНДС", "без НДС")
	}
	return writeTextElement(enc, wrapper, "СумНал", moneyText(vatAmount))
}

func writeVATTotal(enc *xml.Encoder, vatAmount float64, hasVAT bool) error {
	if !hasVAT {
		return writeTextElement(enc, "СумНалВсего", "БезНДС", "без НДС")
	}
	return writeTextElement(enc, "СумНалВсего", "СумНал", moneyText(vatAmount))
}

func customerName(customer models.Customer) string {
	if strings.TrimSpace(customer.Fullname) != "" {
		return customer.Fullname
	}
	return customer.Name
}

func digitsOnly(value string) string {
	re := regexp.MustCompile(`\D+`)
	return re.ReplaceAllString(value, "")
}

func actFileID(customer models.Customer, options ActUPDOptions, formedAt time.Time) (string, error) {
	uid, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("generate document UUID: %w", err)
	}
	return sanitizeFileID(fmt.Sprintf("%s_%s_%s_%s_%s_0_0_0_0_0_00",
		actFilePrefix,
		buyerEDOID(customer),
		sellerParticipantID(options),
		formedAt.Format("20060102"),
		uid,
	)), nil
}

func invoiceFileID(invoice models.InvoiceWithServices, customer models.Customer, docDate time.Time) string {
	raw := fmt.Sprintf("WORKAPP_INVOICE_%s_%s_%s_%s",
		sellerINN,
		digitsOnly(customer.INN),
		strings.TrimSpace(invoice.Number),
		docDate.Format("20060102"),
	)
	return sanitizeFileID(raw)
}

func edoParticipantID(inn, kpp string) string {
	id := digitsOnly(inn)
	if strings.TrimSpace(kpp) != "" {
		id += digitsOnly(kpp)
	}
	return id
}

func buyerEDOID(customer models.Customer) string {
	if id := strings.TrimSpace(customer.EDOIDTensor); id != "" {
		return id
	}
	return edoParticipantID(customer.INN, customer.KPP)
}

func sellerParticipantID(options ActUPDOptions) string {
	if id := strings.TrimSpace(options.SellerEDOID); id != "" {
		return id
	}
	return defaultSellerEDOID
}

func validEDOID(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 || len(trimmed) > 100 {
		return false
	}
	return regexp.MustCompile(`^[A-Za-z0-9-]+$`).MatchString(trimmed)
}

func newUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func moscowNow() time.Time {
	return time.Now().In(time.FixedZone("MSK", 3*60*60))
}

func sanitizeFileID(value string) string {
	raw := regexp.MustCompile(`[^A-Za-zА-Яа-я0-9_-]+`).ReplaceAllString(value, "_")
	return strings.Trim(raw, "_-")
}
