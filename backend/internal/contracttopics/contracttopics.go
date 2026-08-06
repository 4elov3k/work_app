// Package contracttopics is the single source of truth for contract topic
// normalization and validation. Both the REST handlers (internal/handlers)
// and the Hermes MCP service (internal/accounting) map free-form topic input
// to the same canonical set through this package, so a topic that is
// accepted through one channel behaves identically through the other.
package contracttopics

import (
	"fmt"
	"strings"
)

// canonical is the whitelist of valid contract topics.
var canonical = []string{
	"Продвижение сео",
	"Продвижение контекст",
	"Сео + контекст",
	"Техподдержка",
	"Юр услуги",
	"Разработка",
	"Соц сети",
	"Дизайн",
	"Отзывы",
}

// aliases maps free-form/casing variants (already folded via fold) to their
// canonical topic.
var aliases = map[string]string{
	"seo":             "Продвижение сео",
	"сео":             "Продвижение сео",
	"продвижение seo": "Продвижение сео",
	"продвижение сео": "Продвижение сео",
	"контекст":        "Продвижение контекст",
	"продвижение контекст":  "Продвижение контекст",
	"сео + контекст":        "Сео + контекст",
	"seo + контекст":        "Сео + контекст",
	"техподдержка":          "Техподдержка",
	"техническая поддержка": "Техподдержка",
	"юр услуги":             "Юр услуги",
	"юридические услуги":    "Юр услуги",
	"разработка":            "Разработка",
	"разработка сайта":      "Разработка",
	"создание сайта":        "Разработка",
	"сайт":                  "Разработка",
	"соц сети":              "Соц сети",
	"социальные сети":       "Соц сети",
	"дизайн":                "Дизайн",
	"отзывы":                "Отзывы",
}

// Allowed returns the canonical list of valid contract topics.
func Allowed() []string {
	out := make([]string, len(canonical))
	copy(out, canonical)
	return out
}

func fold(topic string) string {
	value := strings.ToLower(strings.TrimSpace(topic))
	value = strings.ReplaceAll(value, "ё", "е")
	return strings.Join(strings.Fields(value), " ")
}

// Normalize maps free-form contract topic input (including known aliases and
// casing/whitespace variants) to its canonical form. It returns an error if
// the topic is empty or does not match any known alias or canonical topic.
func Normalize(topic string) (string, error) {
	value := strings.TrimSpace(topic)
	if value == "" {
		return "", fmt.Errorf("тема договора обязательна")
	}
	if mapped, ok := aliases[fold(value)]; ok {
		return mapped, nil
	}
	for _, allowed := range canonical {
		if value == allowed {
			return value, nil
		}
	}
	return "", fmt.Errorf("некорректная тема договора: %s (используйте одну из: %s)", value, strings.Join(canonical, ", "))
}
