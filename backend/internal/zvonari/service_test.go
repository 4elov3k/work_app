package zvonari

import (
	"encoding/json"
	"testing"

	"invoices-backend/internal/pbx"
)

func TestExtractOutcome(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"normal outcome value", json.RawMessage(`{"outcome":"успешно"}`), "успешно"},
		{"another normal outcome value", json.RawMessage(`{"outcome":"отказ"}`), "отказ"},
		{
			// The legacy pre-regulation shape: analytics_json present but with
			// an explicit empty-string outcome rather than the field being
			// absent or analytics_json being NULL. Backend's SQL-side COALESCE
			// only substitutes on NULL, which is exactly the gap
			// "Звонари: count legacy empty-string outcome in totalUnanalyzed KPI"
			// (544dc82) fixed on the frontend aggregation side. Pin down here
			// that the Go extractor already folds this into the same bucket as
			// "not analyzed" so it can't silently regress.
			"legacy explicit empty-string outcome",
			json.RawMessage(`{"outcome":""}`),
			"не проанализировано",
		},
		{"outcome field missing", json.RawMessage(`{}`), "не проанализировано"},
		{"other fields present, outcome missing", json.RawMessage(`{"call_type":"содержательный"}`), "не проанализировано"},
		{"malformed JSON", json.RawMessage(`{not valid json`), "не проанализировано"},
		{"nil raw message", nil, "не проанализировано"},
		{"empty raw message", json.RawMessage(``), "не проанализировано"},
		{"outcome is not a string", json.RawMessage(`{"outcome":123}`), "не проанализировано"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractOutcome(tt.raw); got != tt.want {
				t.Errorf("ExtractOutcome(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExtractCallType(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"технический", json.RawMessage(`{"call_type":"технический"}`), "технический"},
		{"содержательный", json.RawMessage(`{"call_type":"содержательный"}`), "содержательный"},
		{"недостаточно_данных", json.RawMessage(`{"call_type":"недостаточно_данных"}`), "недостаточно_данных"},
		{
			// Pre-rubric legacy {category, outcome} shape has no call_type at
			// all — must read back as "" (treated the same as unanalyzed by
			// callers), not as some synthetic fourth value.
			"legacy shape without call_type field",
			json.RawMessage(`{"category":"вопрос","outcome":"успешно"}`),
			"",
		},
		{"empty object", json.RawMessage(`{}`), ""},
		{"malformed JSON", json.RawMessage(`{broken`), ""},
		{"nil raw message", nil, ""},
		{"empty raw message", json.RawMessage(``), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractCallType(tt.raw); got != tt.want {
				t.Errorf("ExtractCallType(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExtractFraudSuspected(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{"true", json.RawMessage(`{"fraud_suspected":true}`), true},
		{"false", json.RawMessage(`{"fraud_suspected":false}`), false},
		{"field missing defaults false", json.RawMessage(`{}`), false},
		{"legacy shape without the field", json.RawMessage(`{"outcome":"успешно"}`), false},
		{"malformed JSON defaults false", json.RawMessage(`{oops`), false},
		{"nil raw message defaults false", nil, false},
		{"empty raw message defaults false", json.RawMessage(``), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractFraudSuspected(tt.raw); got != tt.want {
				t.Errorf("ExtractFraudSuspected(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCallerExtension(t *testing.T) {
	tests := []struct {
		name string
		rec  pbx.CallRecord
		want string
	}{
		{
			"outbound call uses caller_id_number regardless of events",
			pbx.CallRecord{
				Accountcode:    "outbound",
				CallerIDNumber: "101",
				Events: []pbx.Event{
					{Type: "user", Number: "202"},
				},
			},
			"101",
		},
		{
			"inbound call uses last user-type event",
			pbx.CallRecord{
				Accountcode: "local",
				Events: []pbx.Event{
					{Type: "user", Number: "101"},
					{Type: "bridge", Number: "999"},
					{Type: "user", Number: "202"},
				},
			},
			"202",
		},
		{
			"no user events at all",
			pbx.CallRecord{
				Accountcode: "local",
				Events: []pbx.Event{
					{Type: "bridge", Number: "999"},
				},
			},
			"",
		},
		{
			"no events at all",
			pbx.CallRecord{Accountcode: "local"},
			"",
		},
		{
			"single user event",
			pbx.CallRecord{
				Accountcode: "inbound",
				Events: []pbx.Event{
					{Type: "user", Number: "303"},
				},
			},
			"303",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := callerExtension(tt.rec); got != tt.want {
				t.Errorf("callerExtension(%+v) = %q, want %q", tt.rec, got, tt.want)
			}
		})
	}
}
