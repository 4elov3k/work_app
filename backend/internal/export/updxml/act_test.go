package updxml

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"invoices-backend/internal/models"
)

func TestBuildActUPDXMLCenterTTMFixture(t *testing.T) {
	act := models.ActWithServices{
		Act: models.Act{
			Number:      "2159",
			Date:        "30.06.2026",
			TotalAmount: 24900,
		},
		Services: []models.Service{{
			Name:  "Услуги по поисковому продвижению сайта за июнь 2026 года",
			Unit:  "шт",
			Price: 24900,
		}},
	}
	customer := models.Customer{
		Name:        "ООО «ЦентрТТМ»",
		Fullname:    "Общество с ограниченной ответственностью «ЦентрТТМ»",
		Address:     "603092, РФ, г. Нижний Новгород, Московское шоссе 302/2, оф. 103",
		INN:         "5257 120323",
		KPP:         "525-701-001",
		EDOIDTensor: "2BE812d49a2f4764a9e8155d95b0ba14708",
	}
	contract := models.Contract{
		Number:    "Основной № 380 от 02.02.2022 г.",
		StartDate: "2022-02-02",
	}

	formedAt := time.Date(2026, 8, 3, 12, 8, 33, 0, time.FixedZone("MSK", 3*60*60))
	data, filename, err := buildActUPDXMLAt(act, customer, contract, sellerActUPDOptions, formedAt)
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
	validateActXMLAgainstFNSXSD(t, data)
	if !strings.HasPrefix(root.FileID, "ON_NSCHFDOPPR_2BE812d49a2f4764a9e8155d95b0ba14708_2BEb25cae8e664f11e38742005056917125_20260803_") {
		t.Fatalf("unexpected file id: %s", root.FileID)
	}
	if !strings.HasSuffix(root.FileID, "_0_0_0_0_0_00") {
		t.Fatalf("file id must contain tensor suffix: %s", root.FileID)
	}
	if filename != root.FileID+".xml" {
		t.Fatalf("filename must match ИдФайл: filename=%s id=%s", filename, root.FileID)
	}
	text := string(data)
	for _, want := range []string{
		`НаимЭконСубСост="Индивидуальный предприниматель Мыленкова Любовь Валерьевна"`,
		`Функция="ДОП"`,
		`НаимДокОпр="Универсальный передаточный документ"`,
		`НомерДок="2159"`,
		`ДатаДок="30.06.2026"`,
		`СодОпер="Оказание услуг"`,
		`<БанкРекв НомерСчета="40802810164270001108">`,
		`НаимБанк="ООО &#34;Банк Точка&#34;"`,
		`<Контакт>`,
		`<Тлф>8-905-864445</Тлф>`,
		`<ДокПодтвОтгрНом`,
		`РеквНаимДок="Универсальный передаточный документ"`,
		`РеквНомерДок="2159"`,
		`РеквДатаДок="30.06.2026"`,
		`<ДенИзм`,
		`КодОКВ="643"`,
		`НаимОКВ="Российский рубль"`,
		`ИННЮЛ="5257120323"`,
		`КПП="525701001"`,
		`НаимОрг="Общество с ограниченной ответственностью «ЦентрТТМ»"`,
		`НаимТов="Услуги по поисковому продвижению сайта за июнь 2026 года"`,
		`НаимЕдИзм="шт"`,
		`ОКЕИ_Тов="796"`,
		`ЦенаТов="24900.00"`,
		`СтТовБезНДС="24900.00"`,
		`НалСт="без НДС"`,
		`СтТовУчНал="24900.00"`,
		`<БезНДС>без НДС</БезНДС>`,
		`СтТовБезНДСВсего="24900.00"`,
		`СтТовУчНалВсего="24900.00"`,
		`<ОснПер`,
		`РеквНаимДок="Договор"`,
		`РеквНомерДок="380"`,
		`РеквДатаДок="02.02.2022"`,
		`Должн="Индивидуальный предприниматель"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated XML does not contain %s", want)
		}
	}
	for _, unwanted := range []string{`Функция="СЧФДОП"`, `<БезДокОснПер>`, `<СумНал>0`, `<СумНал>0.00`, `<ДопСвФХЖ1`, `Счет-фактура на аванс`, `<СопрДокФХЖ`} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("generated XML unexpectedly contains %s", unwanted)
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

func TestBuildActUPDXMLDzerzhinskieVedomostiFixture(t *testing.T) {
	act := models.ActWithServices{
		Act: models.Act{
			Number:      "2161",
			Date:        "31.08.2026",
			TotalAmount: 24900,
		},
		Services: []models.Service{{
			Name:   "Информационное сопровождение за август 2026 года",
			Unit:   "шт",
			Price:  24900,
			Qty:    1,
			Amount: 24900,
		}},
	}
	customer := models.Customer{
		Name:     "МАУ «ИЦ «Дзержинские ведомости»",
		Fullname: "Муниципальное автономное учреждение «Информационный центр «Дзержинские ведомости»",
		Address:  "606000, Нижегородская обл., г. Дзержинск, пр. Дзержинского, д. 9",
		INN:      "5249091492",
		KPP:      "524901001",
	}
	contract := models.Contract{
		Number:    "№ 610 от 22.12.2025",
		StartDate: "2025-12-22",
	}

	data, _, err := BuildActUPDXML(act, customer, contract)
	if err != nil {
		t.Fatalf("BuildActUPDXML returned error: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`ИдФайл="ON_NSCHFDOPPR_5249091492524901001_`,
		`ИННЮЛ="5249091492"`,
		`КПП="524901001"`,
		`НаимОрг="Муниципальное автономное учреждение «Информационный центр «Дзержинские ведомости»"`,
		`РеквНомерДок="610"`,
		`РеквДатаДок="22.12.2025"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated XML does not contain %s", want)
		}
	}
}

func TestBuildActUPDXMLNoVATPrimaryActRules(t *testing.T) {
	data := buildTestActXML(t, models.Contract{}, sellerActUPDOptions)
	doc := parseActXML(t, data)

	if doc.Document.Function != "ДОП" {
		t.Fatalf("function = %q, want ДОП", doc.Document.Function)
	}
	if doc.Document.Function == "СЧФДОП" {
		t.Fatal("no-VAT act must not use invoice-bearing function")
	}
	if len(doc.Document.Table.Rows) == 0 {
		t.Fatal("act must contain service rows")
	}
	for _, row := range doc.Document.Table.Rows {
		if row.VATRate != "без НДС" {
			t.Fatalf("row %s VAT rate = %q, want без НДС", row.Number, row.VATRate)
		}
		if row.VAT.NoVAT != "без НДС" {
			t.Fatalf("row %s VAT marker = %q, want без НДС", row.Number, row.VAT.NoVAT)
		}
		if strings.TrimSpace(row.VAT.Sum) != "" {
			t.Fatalf("row %s must not contain numeric VAT sum %q", row.Number, row.VAT.Sum)
		}
		if row.WithoutVAT != row.WithVAT {
			t.Fatalf("row %s without VAT %s != total %s", row.Number, row.WithoutVAT, row.WithVAT)
		}
	}
	if doc.Document.Table.Total.VAT.NoVAT != "без НДС" {
		t.Fatalf("total VAT marker = %q, want без НДС", doc.Document.Table.Total.VAT.NoVAT)
	}
	if strings.TrimSpace(doc.Document.Table.Total.VAT.Sum) != "" {
		t.Fatalf("total must not contain numeric VAT sum %q", doc.Document.Table.Total.VAT.Sum)
	}
	if doc.Document.Table.Total.WithoutVAT != doc.Document.Table.Total.WithVAT {
		t.Fatalf("total without VAT %s != total %s", doc.Document.Table.Total.WithoutVAT, doc.Document.Table.Total.WithVAT)
	}

	text := string(data)
	for _, unwanted := range []string{`Функция="СЧФДОП"`, `НалСт="0%"`, `НалСт="5%"`, `<СумНал>0`, `<СумНал>0.00`} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("generated XML unexpectedly contains %s", unwanted)
		}
	}
	validateActXMLAgainstFNSXSD(t, data)
}

func TestBuildActUPDXMLWithoutContractUsesNoBasisMarker(t *testing.T) {
	data := buildTestActXML(t, models.Contract{}, sellerActUPDOptions)
	doc := parseActXML(t, data)
	if doc.Document.Transfer.Transfer.NoBasis != "1" {
		t.Fatal("act without contract must contain БезДокОснПер")
	}
	if len(doc.Document.Transfer.Transfer.Bases) != 0 {
		t.Fatal("act without contract must not contain ОснПер")
	}
	validateActXMLAgainstFNSXSD(t, data)
}

func TestBuildActUPDXMLWithContractUsesStructuredBasis(t *testing.T) {
	data := buildTestActXML(t, models.Contract{Number: "Основной № 380 от 02.02.2022 г.", StartDate: "2022-02-02"}, sellerActUPDOptions)
	doc := parseActXML(t, data)
	if doc.Document.Transfer.Transfer.NoBasis != "" {
		t.Fatal("act with contract must not contain БезДокОснПер")
	}
	if len(doc.Document.Transfer.Transfer.Bases) != 1 {
		t.Fatalf("act with contract must contain one ОснПер, got %d", len(doc.Document.Transfer.Transfer.Bases))
	}
	basis := doc.Document.Transfer.Transfer.Bases[0]
	if basis.Name != "Договор" || basis.Number != "380" || basis.Date != "02.02.2022" {
		t.Fatalf("unexpected contract basis: %+v", basis)
	}
	validateActXMLAgainstFNSXSD(t, data)
}

func TestBuildActUPDXMLWithVATAndInvoiceUsesInvoiceFunction(t *testing.T) {
	options := ActUPDOptions{DocumentType: DocumentTypeAct, VATMode: VATModeIncluded, IssueInvoice: true}
	data := buildTestActXML(t, models.Contract{}, options)
	text := string(data)
	for _, want := range []string{`Функция="СЧФДОП"`, `НалСт="5%"`, `ЦенаТов="100.00"`, `СтТовБезНДС="100.00"`, `СтТовУчНал="105.00"`, `<СумНал>5.00</СумНал>`} {
		if !strings.Contains(text, want) {
			t.Fatalf("VAT UPD does not contain %s", want)
		}
	}
	if strings.Contains(text, `<БезНДС>`) {
		t.Fatal("VAT UPD must not contain БезНДС")
	}
	validateActXMLAgainstFNSXSD(t, data)
}

func TestBuildActUPDXMLRejectsInvoiceFlagInNoVATMode(t *testing.T) {
	options := ActUPDOptions{DocumentType: DocumentTypeAct, VATMode: VATModeNone, IssueInvoice: true}
	_, _, err := BuildActUPDXMLWithOptions(testAct(false), testCustomer(), models.Contract{}, options)
	if err == nil || !strings.Contains(err.Error(), "признак выставления счета-фактуры должен быть выключен") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildActUPDXMLRejectsPartialContract(t *testing.T) {
	_, _, err := BuildActUPDXML(testAct(false), testCustomer(), models.Contract{Number: "380"})
	if err == nil || !strings.Contains(err.Error(), "contract number and date must be provided together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildActUPDXMLRejectsVATInNoVATMode(t *testing.T) {
	_, _, err := BuildActUPDXML(testAct(true), testCustomer(), models.Contract{})
	if err == nil || !strings.Contains(err.Error(), "VAT rate must be zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildActUPDXMLUsesFreshFileID(t *testing.T) {
	_, first, err := BuildActUPDXML(testAct(false), testCustomer(), models.Contract{})
	if err != nil {
		t.Fatalf("first BuildActUPDXML returned error: %v", err)
	}
	_, second, err := BuildActUPDXML(testAct(false), testCustomer(), models.Contract{})
	if err != nil {
		t.Fatalf("second BuildActUPDXML returned error: %v", err)
	}
	if first == second {
		t.Fatalf("file IDs must be unique, both were %s", first)
	}
}

func TestBuildActUPDXMLRejectsTotalMismatch(t *testing.T) {
	act := testAct(false)
	act.TotalAmount = 99
	_, _, err := BuildActUPDXML(act, testCustomer(), models.Contract{})
	if err == nil || !strings.Contains(err.Error(), "total amount must equal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildActUPDXMLQuotesSpecialCharacters(t *testing.T) {
	act := testAct(false)
	act.Services[0].Name = `SEO & аудит "сайта" <июнь>`
	data, _, err := BuildActUPDXML(act, testCustomer(), models.Contract{})
	if err != nil {
		t.Fatalf("BuildActUPDXML returned error: %v", err)
	}
	var parsed any
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("XML with special characters is not well-formed: %v", err)
	}
	if !strings.Contains(string(data), `SEO &amp; аудит &#34;сайта&#34; &lt;июнь&gt;`) {
		t.Fatalf("special characters were not escaped: %s", data)
	}
}

func buildTestActXML(t *testing.T, contract models.Contract, options ActUPDOptions) []byte {
	t.Helper()
	act := testAct(options.VATMode == VATModeIncluded)
	data, _, err := buildActUPDXMLAt(act, testCustomer(), contract, options, time.Date(2026, 8, 3, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60)))
	if err != nil {
		t.Fatalf("BuildActUPDXML returned error: %v", err)
	}
	return data
}

func testAct(withVAT bool) models.ActWithServices {
	service := models.Service{Name: "Услуги по продвижению сайта за июнь 2026 года", Unit: "шт", Price: 100, Qty: 1, Amount: 100}
	total := 100.0
	if withVAT {
		service.VAT = 5
		service.Price = 105
		service.Amount = 105
		total = 105
	}
	return models.ActWithServices{Act: models.Act{Number: "2159", Date: "30.06.2026", TotalAmount: total}, Services: []models.Service{service}}
}

func testCustomer() models.Customer {
	return models.Customer{Name: "ООО «ЦентрТТМ»", Fullname: "Общество с ограниченной ответственностью «ЦентрТТМ»", Address: "Нижний Новгород & область", INN: "5257120323", KPP: "525701001", EDOIDTensor: "2BE812d49a2f4764a9e8155d95b0ba14708"}
}

func validateActXMLAgainstFNSXSD(t *testing.T, data []byte) {
	t.Helper()
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint is required for strict FNS XSD validation")
	}
	xsdPath := filepath.Join("schema", "fns-2026-01", "ON_NSCHFDOPPR_1_997_01_05_03_05.xsd")
	xmlPath := filepath.Join(t.TempDir(), "upd.xml")
	if err := os.WriteFile(xmlPath, data, 0o600); err != nil {
		t.Fatalf("write temporary XML: %v", err)
	}
	output, err := exec.Command(xmllint, "--noout", "--schema", xsdPath, xmlPath).CombinedOutput()
	if err != nil {
		t.Fatalf("generated XML does not match official FNS XSD: %v\n%s", err, output)
	}
}

type parsedActXML struct {
	FileID   string `xml:"ИдФайл,attr"`
	Document struct {
		Function string `xml:"Функция,attr"`
		Table    struct {
			Rows []struct {
				Number     string `xml:"НомСтр,attr"`
				VATRate    string `xml:"НалСт,attr"`
				WithoutVAT string `xml:"СтТовБезНДС,attr"`
				WithVAT    string `xml:"СтТовУчНал,attr"`
				VAT        struct {
					NoVAT string `xml:"БезНДС"`
					Sum   string `xml:"СумНал"`
				} `xml:"СумНал"`
			} `xml:"СведТов"`
			Total struct {
				WithoutVAT string `xml:"СтТовБезНДСВсего,attr"`
				WithVAT    string `xml:"СтТовУчНалВсего,attr"`
				VAT        struct {
					NoVAT string `xml:"БезНДС"`
					Sum   string `xml:"СумНал"`
				} `xml:"СумНалВсего"`
			} `xml:"ВсегоОпл"`
		} `xml:"ТаблСчФакт"`
		Transfer struct {
			Transfer struct {
				Operation string `xml:"СодОпер,attr"`
				Bases     []struct {
					Name   string `xml:"РеквНаимДок,attr"`
					Number string `xml:"РеквНомерДок,attr"`
					Date   string `xml:"РеквДатаДок,attr"`
				} `xml:"ОснПер"`
				NoBasis string `xml:"БезДокОснПер"`
			} `xml:"СвПер"`
		} `xml:"СвПродПер"`
	} `xml:"Документ"`
}

func parseActXML(t *testing.T, data []byte) parsedActXML {
	t.Helper()
	var doc parsedActXML
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("generated XML is not well-formed: %v", err)
	}
	return doc
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
