package updxml

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invoices-backend/internal/models"
)

func TestBuildActUPDXMLCenterTTMFixture(t *testing.T) {
	act := models.ActWithServices{
		Act: models.Act{
			Number: "2051",
			Date:   "30.01.2026",
		},
		Services: []models.Service{{
			Name:  "Ежемесячное продвижение сайта в ТОП 10 Яндекс за январь 2026 года",
			Price: 24900,
		}},
		Invoices: []models.Invoice{{
			Number:      "2051",
			Date:        "30.01.2026",
			TotalAmount: 24900,
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

	data, filename, err := BuildActUPDXML(act, customer, contract)
	if err != nil {
		t.Fatalf("BuildActUPDXML returned error: %v", err)
	}
	var root struct {
		XMLName xml.Name
		FileID  string `xml:"ИдФайл,attr"`
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatalf("generated XML is not well-formed: %v", err)
	}
	if !strings.HasPrefix(root.FileID, "ON_NSCHFDOPPR_526220116209_5257120323525701001_20260130_2051_20260130_") {
		t.Fatalf("unexpected file id: %s", root.FileID)
	}
	if filename != root.FileID+".xml" {
		t.Fatalf("filename must match ИдФайл: filename=%s id=%s", filename, root.FileID)
	}
	text := string(data)
	for _, want := range []string{
		`НаимЭконСубСост="Индивидуальный предприниматель Мыленкова Любовь Валерьевна"`,
		`НомерДок="2051"`,
		`ДатаДок="30.01.2026"`,
		`<СвПРД`,
		`НомерПРД="2051"`,
		`ДатаПРД="30.01.2026"`,
		`СуммаПРД="24900.00"`,
		`РеквНаимДок="Акт выполненных работ"`,
		`РеквНомерДок="2051"`,
		`РеквДатаДок="30.01.2026"`,
		`<ДокПодтвОтгрНом`,
		`РеквНаимДок="Акт выполненных работ"`,
		`РеквНомерДок="2051"`,
		`РеквДатаДок="30.01.2026"`,
		`<ДенИзм`,
		`КодОКВ="643"`,
		`НаимОКВ="Российский рубль"`,
		`ИННЮЛ="5257120323"`,
		`КПП="525701001"`,
		`НаимОрг="Общество с ограниченной ответственностью «ЦентрТТМ»"`,
		`<ИнфПолФХЖ1>`,
		`<ТекстИнф`,
		`Идентиф="Договор"`,
		`СтТовУчНалВсего="24900.00"`,
		`<Подписант СпосПодтПолном="1">`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated XML does not contain %s", want)
		}
	}

	if os.Getenv("UPDXML_WRITE_FIXTURE") == "1" {
		outPath := filepath.Join("..", "..", "..", "..", "export", filename)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			t.Fatalf("failed to create export dir: %v", err)
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			t.Fatalf("failed to write fixture XML: %v", err)
		}
		t.Logf("wrote %s", outPath)
	}
}

func TestBuildActUPDXMLRejectsLineAmountMismatch(t *testing.T) {
	act := models.ActWithServices{
		Act: models.Act{
			Number: "2052",
			Date:   "30.01.2026",
		},
		Services: []models.Service{{
			Name:   "Ежемесячное продвижение сайта",
			Price:  100,
			Qty:    3,
			Amount: 301,
		}},
	}
	customer := models.Customer{
		Name:     "ООО «ЦентрТТМ»",
		Fullname: "Общество с ограниченной ответственностью «ЦентрТТМ»",
		Address:  "603092, РФ, г. Нижний Новгород, Московское шоссе 302/2, оф. 103",
		INN:      "5257120323",
		KPP:      "525701001",
	}

	_, _, err := BuildActUPDXML(act, customer, models.Contract{})
	if err == nil {
		t.Fatal("expected amount mismatch error")
	}
	if !strings.Contains(err.Error(), "amount must equal price multiplied by quantity") {
		t.Fatalf("unexpected error: %v", err)
	}
}
