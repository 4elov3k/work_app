package updxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"invoices-backend/internal/models"
)

// BuildInvoiceXML builds a machine-readable XML export for work_app invoices.
// A payment invoice is not a formalized FNS UPD document, but the export keeps
// the same counterparty and table structure used by the act XML export.
func BuildInvoiceXML(invoice models.InvoiceWithServices, customer models.Customer, contract models.Contract) ([]byte, string, error) {
	if strings.TrimSpace(invoice.Number) == "" {
		return nil, "", fmt.Errorf("invoice number is required")
	}
	if strings.TrimSpace(invoice.Date) == "" {
		return nil, "", fmt.Errorf("invoice date is required")
	}
	if err := validateCustomer(customer); err != nil {
		return nil, "", err
	}
	if len(invoice.Services) == 0 {
		return nil, "", fmt.Errorf("invoice has no service lines")
	}
	if err := validateServicesForEDO(invoice.Services); err != nil {
		return nil, "", err
	}

	docDate, err := parseRuDate(invoice.Date)
	if err != nil {
		return nil, "", err
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	enc := xml.NewEncoder(&b)
	enc.Indent("", "  ")

	fileID := invoiceFileID(invoice, customer, docDate)
	root := xml.StartElement{
		Name: xml.Name{Local: "Файл"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "ИдФайл"}, Value: fileID},
			{Name: xml.Name{Local: "ВерсПрог"}, Value: "work_app"},
			{Name: xml.Name{Local: "ВерсФорм"}, Value: "work_app-invoice"},
		},
	}
	if err := enc.EncodeToken(root); err != nil {
		return nil, "", err
	}
	if err := writeInvoiceDocument(enc, invoice, customer, contract, docDate); err != nil {
		return nil, "", err
	}
	if err := enc.EncodeToken(root.End()); err != nil {
		return nil, "", err
	}
	if err := enc.Flush(); err != nil {
		return nil, "", err
	}
	return b.Bytes(), invoiceXMLFilename(invoice), nil
}

func writeInvoiceDocument(enc *xml.Encoder, invoice models.InvoiceWithServices, customer models.Customer, contract models.Contract, docDate time.Time) error {
	now := time.Now()
	doc := xml.StartElement{
		Name: xml.Name{Local: "Документ"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "ДатаИнфПр"}, Value: now.Format("02.01.2006")},
			{Name: xml.Name{Local: "ВремИнфПр"}, Value: now.Format("15.04.05")},
			{Name: xml.Name{Local: "НаимЭконСубСост"}, Value: sellerFullName},
			{Name: xml.Name{Local: "НаимДокОпр"}, Value: "Счет на оплату"},
		},
	}
	if err := enc.EncodeToken(doc); err != nil {
		return err
	}
	if err := writePaymentInvoiceInfo(enc, invoice, customer, contract, docDate); err != nil {
		return err
	}
	if err := writeTable(enc, invoice.Services); err != nil {
		return err
	}
	if err := writeSigner(enc); err != nil {
		return err
	}
	return enc.EncodeToken(doc.End())
}

func writePaymentInvoiceInfo(enc *xml.Encoder, invoice models.InvoiceWithServices, customer models.Customer, contract models.Contract, docDate time.Time) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "СвСчет"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "Номер"}, Value: invoice.Number},
			{Name: xml.Name{Local: "Дата"}, Value: docDate.Format("02.01.2006")},
			{Name: xml.Name{Local: "КодОКВ"}, Value: "643"},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := writeSeller(enc); err != nil {
		return err
	}
	if err := writeBuyer(enc, customer); err != nil {
		return err
	}
	if strings.TrimSpace(contract.Number) != "" {
		if err := writeSimpleElement(enc, "ИнфПолФХЖ1", map[string]string{
			"Идентиф": "Договор",
			"Значен":  contract.Number,
		}); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}

func invoiceXMLFilename(invoice models.InvoiceWithServices) string {
	return fmt.Sprintf("Счет № %s от %s.xml", strings.TrimSpace(invoice.Number), strings.TrimSpace(invoice.Date))
}
