package accounting

import "testing"

func TestCanonicalJSONHashSurvivesJSONBFormatting(t *testing.T) {
	payload := map[string]any{
		"input": CreateCounterpartyInput{
			Type:          "company",
			Name:          "МАУ «ИЦ «Дзержинские ведомости»",
			FullName:      "Муниципальное автономное учреждение «Информационный центр «Дзержинские ведомости»",
			INN:           "5249091492",
			KPP:           "524901001",
			Email:         "dzved@mail.ru",
			Phone:         "(8313) 27-99-79",
			Address:       "606000, Нижегородская обл., г. Дзержинск, пр. Дзержинского, д. 9",
			ContactPerson: "Дмитрий Касаткин",
			Comment:       "Источник: договор №610 от 22.12.2025, скан договора",
		},
	}
	prepared, err := canonicalJSONBytes(payload)
	if err != nil {
		t.Fatal(err)
	}

	fromJSONB := []byte(`{"input": {"inn": "5249091492", "kpp": "524901001", "name": "МАУ «ИЦ «Дзержинские ведомости»", "type": "company", "email": "dzved@mail.ru", "phone": "(8313) 27-99-79", "address": "606000, Нижегородская обл., г. Дзержинск, пр. Дзержинского, д. 9", "comment": "Источник: договор №610 от 22.12.2025, скан договора", "fullname": "Муниципальное автономное учреждение «Информационный центр «Дзержинские ведомости»", "contact_person": "Дмитрий Касаткин"}}`)
	committed, err := canonicalJSONBytes(fromJSONB)
	if err != nil {
		t.Fatal(err)
	}

	if hashBytes(prepared) != hashBytes(committed) {
		t.Fatalf("canonical payload hash changed after jsonb formatting\nprepared:  %s\ncommitted: %s", string(prepared), string(committed))
	}
}

func TestNormalizeContractTopic(t *testing.T) {
	tests := map[string]string{
		"Разработка":       "Разработка",
		"Разработка сайта": "Разработка",
		"создание сайта":   "Разработка",
		"seo":              "Продвижение сео",
		"продвижение seo":  "Продвижение сео",
	}

	for input, want := range tests {
		got, err := normalizeContractTopic(input)
		if err != nil {
			t.Fatalf("normalizeContractTopic(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeContractTopic(%q) = %q, want %q", input, got, want)
		}
	}
}
