package docparse

import "testing"

// Real OCR output from the digitized Договор №602 scan (МАУ «ИЦ «Дзержинские
// ведомости»), including the glued-together "02Декабря" that tesseract
// produces when the scan has no space between day and month.
const sampleContractText = `ДОГОВОР №602
г. Нижний Новгород 02Декабря 2025 г.

Индивидуальный предприниматель Мыленкова Любовь Валерьевна, свидетельство серия 52 №004600460 выдано 27
сентября 2012 года Инспекцией Федеральной налоговой службы по Советскому району г. Нижнего Новгорода, именуемая в
дальнейшем «Исполнитель», с одной стороны,и

муниципальное автономное учреждение Информационный центр «Дзержинские ведомости»в лице и.о.директора
Липатовой Анастасии Павловны, действующей на основании Распоряжения администрации г.Дзержинска.
ИНН: 526220116209
ИНН 5249091492/KNN 524901001`

func TestExtractContractFields(t *testing.T) {
	fields := ExtractContractFields(sampleContractText, "526220116209")

	if fields.Number != "602" {
		t.Errorf("Number = %q, want 602", fields.Number)
	}
	if fields.Date != "2025-12-02" {
		t.Errorf("Date = %q, want 2025-12-02", fields.Date)
	}
	if len(fields.CandidateINN) != 1 || fields.CandidateINN[0] != "5249091492" {
		t.Errorf("CandidateINN = %v, want [5249091492] (seller INN excluded)", fields.CandidateINN)
	}
}

func TestExtractContractFields_DigitDateFallback(t *testing.T) {
	fields := ExtractContractFields("Договор № 15 от 03.07.2024 г.", "")
	if fields.Number != "15" {
		t.Errorf("Number = %q, want 15", fields.Number)
	}
	if fields.Date != "2024-07-03" {
		t.Errorf("Date = %q, want 2024-07-03", fields.Date)
	}
}

func TestExtractContractFields_NoMatch(t *testing.T) {
	fields := ExtractContractFields("просто какой-то текст без договора", "")
	if fields.Number != "" || fields.Date != "" || fields.CandidateINN != nil {
		t.Errorf("expected all-empty fields, got %+v", fields)
	}
}

func TestMonthNumber_DoesNotConfuseMartAndMay(t *testing.T) {
	if num, ok := monthNumber("Марта"); !ok || num != "03" {
		t.Errorf("Марта -> %q, %v; want 03, true", num, ok)
	}
	if num, ok := monthNumber("Мая"); !ok || num != "05" {
		t.Errorf("Мая -> %q, %v; want 05, true", num, ok)
	}
}
