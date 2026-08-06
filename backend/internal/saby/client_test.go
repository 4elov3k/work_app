package saby

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestLookupParticipantIDUsesPrimaryExtendedIdentifier(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://example.test/service" {
				t.Fatalf("unexpected URL %q", r.URL.String())
			}
			if got := r.Header.Get("X-SBISAccessToken"); got != "token" {
				t.Fatalf("expected access token header, got %q", got)
			}

			var req rpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req.Method != "СБИС.ИнформацияОКонтрагенте" {
				t.Fatalf("unexpected method %q", req.Method)
			}

			return jsonResponse(`{
				"jsonrpc": "2.0",
				"result": {
					"Идентификатор": [
						{"ИдентификаторУчастника": "2BM-5257120323-525701001-201609070707163473371", "Основной": "Нет", "СостояниеПодключения": {"Код": "0"}},
						{"ИдентификаторУчастника": "2BE812d49a2f4764a9e8155d95b0ba14708", "Основной": "Да", "СостояниеПодключения": {"Код": "0"}}
					]
				},
				"id": 1
			}`)
		})},
		serviceURL:  "https://example.test/service",
		accessToken: "token",
	}

	id, err := client.LookupParticipantID(context.Background(), Party{
		INN:  "5257120323",
		KPP:  "525701001",
		Name: "ООО ЦентрТТМ",
	})
	if err != nil {
		t.Fatalf("LookupParticipantID returned error: %v", err)
	}
	if id != "2BE812d49a2f4764a9e8155d95b0ba14708" {
		t.Fatalf("unexpected participant ID %q", id)
	}
}

func TestLookupParticipantIDUsesStringIdentifier(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("X-SBISSessionID"); got != "session" {
				t.Fatalf("expected session header, got %q", got)
			}
			return jsonResponse(`{
				"jsonrpc": "2.0",
				"result": {"Идентификатор": "2BE-c662b8816e224776a9b8517b9bacc8b4"},
				"id": 1
			}`)
		})},
		serviceURL: "https://example.test/service",
		sessionID:  "session",
	}

	id, err := client.LookupParticipantID(context.Background(), Party{INN: "526220116209"})
	if err != nil {
		t.Fatalf("LookupParticipantID returned error: %v", err)
	}
	if id != "2BE-c662b8816e224776a9b8517b9bacc8b4" {
		t.Fatalf("unexpected participant ID %q", id)
	}
}

func TestLookupParticipantIDRequiresKPPForOrganization(t *testing.T) {
	client := &Client{accessToken: "token"}

	_, err := client.LookupParticipantID(context.Background(), Party{INN: "5257120323"})
	if err == nil {
		t.Fatal("expected KPP validation error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}, nil
}
