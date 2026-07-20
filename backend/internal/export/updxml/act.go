package updxml

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"invoices-backend/internal/models"
)

const (
	actFilePrefix    = "ON_NSCHFDOPPR"
	sellerFullName   = "Индивидуальный предприниматель Мыленкова Любовь Валерьевна"
	sellerINN        = "526220116209"
	sellerOGRNIP     = "312526227100047"
	sellerAddress    = "603136, г. Нижний Новгород ул, Маршала Рокоссовского, д. 2к1, кв 135"
	sellerLastName   = "Мыленкова"
	sellerFirstName  = "Любовь"
	sellerMiddleName = "Валерьевна"
)

// BuildActUPDXML builds a machine-readable XML export for Saby/1C import.
// It is limited to work_app acts for services, without VAT.
func BuildActUPDXML(act models.ActWithServices, customer models.Customer, contract models.Contract) ([]byte, string, error) {
	if strings.TrimSpace(act.Number) == "" {
		return nil, "", fmt.Errorf("act number is required")
	}
	if strings.TrimSpace(act.Date) == "" {
		return nil, "", fmt.Errorf("act date is required")
	}
	if err := validateCustomer(customer); err != nil {
		return nil, "", err
	}
	if len(act.Services) == 0 {
		return nil, "", fmt.Errorf("act has no service lines")
	}
	if err := validateServicesForEDO(act.Services); err != nil {
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

	fileID := actFileID(act, customer, docDate)
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
	if err := writeDocument(enc, act, customer, contract, docDate); err != nil {
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

func writeDocument(enc *xml.Encoder, act models.ActWithServices, customer models.Customer, contract models.Contract, docDate time.Time) error {
	now := time.Now()
	doc := xml.StartElement{
		Name: xml.Name{Local: "Документ"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "КНД"}, Value: "1115131"},
			{Name: xml.Name{Local: "Функция"}, Value: "ДОП"},
			{Name: xml.Name{Local: "ДатаИнфПр"}, Value: now.Format("02.01.2006")},
			{Name: xml.Name{Local: "ВремИнфПр"}, Value: now.Format("15.04.05")},
			{Name: xml.Name{Local: "НаимЭконСубСост"}, Value: sellerFullName},
			{Name: xml.Name{Local: "ПоФактХЖ"}, Value: "Документ об оказании услуг"},
			{Name: xml.Name{Local: "НаимДокОпр"}, Value: "Акт выполненных работ"},
		},
	}
	if err := enc.EncodeToken(doc); err != nil {
		return err
	}
	if err := writeInvoiceInfo(enc, act, customer, contract, docDate); err != nil {
		return err
	}
	if err := writeTable(enc, act.Services); err != nil {
		return err
	}
	if err := writeTransferInfo(enc, act, docDate); err != nil {
		return err
	}
	if err := writeSigner(enc); err != nil {
		return err
	}
	return enc.EncodeToken(doc.End())
}

func writeInvoiceInfo(enc *xml.Encoder, act models.ActWithServices, customer models.Customer, contract models.Contract, docDate time.Time) error {
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
	if err := writePaymentDocuments(enc, act.Invoices); err != nil {
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
	if strings.TrimSpace(contract.Number) != "" {
		if err := writeInfoField1(enc, "Договор", strings.TrimSpace(contract.Number)); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}

func writePaymentDocuments(enc *xml.Encoder, invoices []models.Invoice) error {
	for _, invoice := range invoices {
		number := strings.TrimSpace(invoice.Number)
		date := strings.TrimSpace(invoice.Date)
		if number == "" || date == "" {
			continue
		}
		parsedDate, err := parseRuDate(date)
		if err != nil {
			return fmt.Errorf("invalid linked invoice date %q: expected DD.MM.YYYY", date)
		}
		attrs := map[string]string{
			"НомерПРД": number,
			"ДатаПРД":  parsedDate.Format("02.01.2006"),
		}
		if invoice.TotalAmount > 0 {
			attrs["СуммаПРД"] = moneyText(invoice.TotalAmount)
		}
		if err := writeSimpleElement(enc, "СвПРД", attrs); err != nil {
			return err
		}
	}
	return nil
}

func writeShipmentDocumentInfo(enc *xml.Encoder, act models.ActWithServices, docDate time.Time) error {
	return writeSimpleElement(enc, "ДокПодтвОтгрНом", map[string]string{
		"РеквНаимДок":  "Акт выполненных работ",
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

func writeInfoField1(enc *xml.Encoder, id, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: "ИнфПолФХЖ1"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeSimpleElement(enc, "ТекстИнф", map[string]string{
		"Идентиф": id,
		"Значен":  value,
	}); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
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
	return enc.EncodeToken(start.End())
}

func writeBuyer(enc *xml.Encoder, customer models.Customer) error {
	start := xml.StartElement{Name: xml.Name{Local: "СвПокуп"}}
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

func writeTable(enc *xml.Encoder, services []models.Service) error {
	start := xml.StartElement{Name: xml.Name{Local: "ТаблСчФакт"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	total := 0.0
	for index, service := range services {
		qty, unitPrice, amount := serviceLineNumbers(service)
		total += amount
		row := xml.StartElement{
			Name: xml.Name{Local: "СведТов"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "НомСтр"}, Value: fmt.Sprint(index + 1)},
				{Name: xml.Name{Local: "НаимТов"}, Value: strings.TrimSpace(service.Name)},
				{Name: xml.Name{Local: "ОКЕИ_Тов"}, Value: "796"},
				{Name: xml.Name{Local: "КолТов"}, Value: quantityText(qty)},
				{Name: xml.Name{Local: "ЦенаТов"}, Value: moneyText(unitPrice)},
				{Name: xml.Name{Local: "СтТовБезНДС"}, Value: moneyText(amount)},
				{Name: xml.Name{Local: "НалСт"}, Value: "без НДС"},
				{Name: xml.Name{Local: "СтТовУчНал"}, Value: moneyText(amount)},
			},
		}
		if err := enc.EncodeToken(row); err != nil {
			return err
		}
		if err := writeTextElement(enc, "Акциз", "БезАкциз", "без акциза"); err != nil {
			return err
		}
		if err := writeTextElement(enc, "СумНал", "БезНДС", "без НДС"); err != nil {
			return err
		}
		if err := enc.EncodeToken(row.End()); err != nil {
			return err
		}
	}
	total = money(total)
	totalStart := xml.StartElement{
		Name: xml.Name{Local: "ВсегоОпл"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "СтТовБезНДСВсего"}, Value: moneyText(total)},
			{Name: xml.Name{Local: "СтТовУчНалВсего"}, Value: moneyText(total)},
		},
	}
	if err := enc.EncodeToken(totalStart); err != nil {
		return err
	}
	if err := writeTextElement(enc, "СумНалВсего", "БезНДС", "без НДС"); err != nil {
		return err
	}
	if err := enc.EncodeToken(totalStart.End()); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func writeTransferInfo(enc *xml.Encoder, act models.ActWithServices, docDate time.Time) error {
	start := xml.StartElement{Name: xml.Name{Local: "СвПродПер"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}

	transfer := xml.StartElement{
		Name: xml.Name{Local: "СвПер"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "СодОпер"}, Value: "Услуги оказаны"},
			{Name: xml.Name{Local: "ДатаПер"}, Value: docDate.Format("02.01.2006")},
		},
	}
	if err := enc.EncodeToken(transfer); err != nil {
		return err
	}
	if err := writeSimpleElement(enc, "ОснПер", map[string]string{
		"РеквНаимДок":  "Акт выполненных работ",
		"РеквНомерДок": strings.TrimSpace(act.Number),
		"РеквДатаДок":  docDate.Format("02.01.2006"),
	}); err != nil {
		return err
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

func parseRuDate(value string) (time.Time, error) {
	parsed, err := time.Parse("02.01.2006", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid act date %q: expected DD.MM.YYYY", value)
	}
	return parsed, nil
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

func validateServicesForEDO(services []models.Service) error {
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
		if money(unitPrice*qty) != amount {
			return fmt.Errorf("service line %d amount must equal price multiplied by quantity", lineNumber)
		}
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

func actFileID(act models.ActWithServices, customer models.Customer, docDate time.Time) string {
	raw := fmt.Sprintf("%s_%s_%s_%s_%s",
		actFilePrefix,
		edoParticipantID(sellerINN, ""),
		edoParticipantID(customer.INN, customer.KPP),
		docDate.Format("20060102"),
		documentUID(act.Number, docDate),
	)
	return sanitizeFileID(raw)
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

func documentUID(number string, docDate time.Time) string {
	seed := sanitizeFileID(fmt.Sprintf("%s_%s", strings.TrimSpace(number), docDate.Format("20060102")))
	sum := sha256.Sum256([]byte(seed))
	return seed + "_" + strings.ToUpper(hex.EncodeToString(sum[:6]))
}

func sanitizeFileID(value string) string {
	raw := strings.ToUpper(regexp.MustCompile(`[^A-ZА-Я0-9_]+`).ReplaceAllString(value, "_"))
	return strings.Trim(raw, "_")
}
