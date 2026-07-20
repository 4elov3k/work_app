package updxml

import (
	"encoding/xml"
	"strings"
	"testing"

	"invoices-backend/internal/models"
)

func TestBuildInvoiceXMLCenterTTMFixture(t *testing.T) {
	invoice := models.InvoiceWithServices{
		Invoice: models.Invoice{
			Number: "2321",
			Date:   "04.02.2026",
		},
		Services: []models.Service{{
			Name:  "Ежемесячное продвижение сайта в ТОП 10 Яндекс. За февраль 2026 года",
			Price: 24900,
		}},
	}
	customer := models.Customer{
		Name:     "ООО «ЦентрТТМ»",
		Fullname: "Общество с ограниченной ответственностью «ЦентрТТМ»",
		Address:  "603092, РФ, г. Нижний Новгород, Московское шоссе 302/2, оф. 103",
		INN:      "5257 120323",
		KPP:      "525-701-001",
	}
	contract := models.Contract{
		Number: "Основной № 380 от 02.02.2022 г.",
	}

	data, filename, err := BuildInvoiceXML(invoice, customer, contract)
	if err != nil {
		t.Fatalf("BuildInvoiceXML returned error: %v", err)
	}
	if filename != "Счет № 2321 от 04.02.2026.xml" {
		t.Fatalf("unexpected filename: %s", filename)
	}
	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatalf("generated XML is not well-formed: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`ИдФайл="WORKAPP_INVOICE_526220116209_5257120323_2321_20260204"`,
		`НаимЭконСубСост="Индивидуальный предприниматель Мыленкова Любовь Валерьевна"`,
		`Номер="2321"`,
		`КПП="525701001"`,
		`НаимТов="Ежемесячное продвижение сайта в ТОП 10 Яндекс. За февраль 2026 года"`,
		`СтТовУчНалВсего="24900.00"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated XML does not contain %s", want)
		}
	}
}
