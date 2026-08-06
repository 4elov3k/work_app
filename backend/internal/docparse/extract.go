package docparse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	contractNumberRe = regexp.MustCompile(`(?i)договор[а-я]*\s*№\s*([0-9]+)`)
	dateDigitsRe     = regexp.MustCompile(`([0-9]{1,2})[./]([0-9]{1,2})[./]([0-9]{4})`)
	dateRussianRe    = regexp.MustCompile(`(?i)([0-9]{1,2})\s*(январ[а-я]*|феврал[а-я]*|март[а-я]*|апрел[а-я]*|ма[йя][а-я]*|июн[а-я]*|июл[а-я]*|август[а-я]*|сентябр[а-я]*|октябр[а-я]*|ноябр[а-я]*|декабр[а-я]*)\s*([0-9]{4})`)
	// {10,12} greedily grabs the longest run so a 12-digit INN isn't cut down
	// to its first 10 digits by an alternation that tries the shorter form first.
	innRe = regexp.MustCompile(`(?i)ИНН[\s:]*([0-9]{10,12})`)

	// Keyed by the first 3 runes of the (lowercased) matched month word. Every
	// Russian month name/declension pair is unambiguous at 3 runes — unlike a
	// naive 2-rune prefix, where "март" and "май"/"мая" would collide.
	russianMonthsByPrefix = map[string]string{
		"янв": "01", "фев": "02", "мар": "03", "апр": "04",
		"май": "05", "мая": "05", "июн": "06", "июл": "07", "авг": "08",
		"сен": "09", "окт": "10", "ноя": "11", "дек": "12",
	}
)

// ContractFields holds the handful of contract-header fields work_app's
// "create contract" form can prefill from an uploaded scan. Fields left
// empty simply weren't found — the caller must not invent a fallback and
// should let the user fill them in manually (see work-app-contract-parsing
// skill's "never default the number to a placeholder" rule, which applies
// here too).
type ContractFields struct {
	Number       string   `json:"number,omitempty"`
	Date         string   `json:"date,omitempty"` // YYYY-MM-DD, for an <input type="date">
	CandidateINN []string `json:"candidate_inn,omitempty"`
}

func monthNumber(name string) (string, bool) {
	runes := []rune(strings.ToLower(name))
	if len(runes) < 3 {
		return "", false
	}
	num, ok := russianMonthsByPrefix[string(runes[:3])]
	return num, ok
}

// ExtractContractFields runs best-effort regex extraction over OCR'd/text-layer
// contract text. It never guesses — a field is only set when a clear match was
// found in the text.
func ExtractContractFields(text string, excludeINN string) ContractFields {
	var fields ContractFields

	if m := contractNumberRe.FindStringSubmatch(text); m != nil {
		fields.Number = m[1]
	}

	if m := dateRussianRe.FindStringSubmatch(text); m != nil {
		day, _ := strconv.Atoi(m[1])
		if month, ok := monthNumber(m[2]); ok && day >= 1 && day <= 31 {
			fields.Date = fmt.Sprintf("%s-%s-%02d", m[3], month, day)
		}
	}
	if fields.Date == "" {
		if m := dateDigitsRe.FindStringSubmatch(text); m != nil {
			day, _ := strconv.Atoi(m[1])
			month, _ := strconv.Atoi(m[2])
			if day >= 1 && day <= 31 && month >= 1 && month <= 12 {
				fields.Date = fmt.Sprintf("%s-%02d-%02d", m[3], month, day)
			}
		}
	}

	seen := map[string]bool{}
	for _, m := range innRe.FindAllStringSubmatch(text, -1) {
		inn := m[1]
		if len(inn) != 10 && len(inn) != 12 {
			continue
		}
		if inn == excludeINN || seen[inn] {
			continue
		}
		seen[inn] = true
		fields.CandidateINN = append(fields.CandidateINN, inn)
	}

	return fields
}
