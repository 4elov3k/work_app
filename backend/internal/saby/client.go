package saby

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://online.sbis.ru"
	authPath       = "/auth/service/"
	servicePath    = "/service/?srv=1"
)

var errNotConfigured = errors.New("saby client is not configured")

type Client struct {
	httpClient    *http.Client
	authURL       string
	serviceURL    string
	accessToken   string
	sessionID     string
	login         string
	password      string
	accountNumber string
	mu            sync.Mutex
}

type Party struct {
	INN  string
	KPP  string
	Name string
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Details string          `json:"details"`
	Data    json.RawMessage `json:"data"`
}

type participantInfo struct {
	Identifier participantIdentifier `json:"Идентификатор"`
}

type participantIdentifier struct {
	Value string
	List  []participantIdentifierItem
}

type participantIdentifierItem struct {
	ParticipantID string `json:"ИдентификаторУчастника"`
	Primary       string `json:"Основной"`
	Roaming       string `json:"Роуминг"`
	Operator      struct {
		ID   string `json:"Идентификатор"`
		Name string `json:"Название"`
	} `json:"Оператор"`
	ConnectionState struct {
		Code        string `json:"Код"`
		Description string `json:"Описание"`
	} `json:"СостояниеПодключения"`
}

func (id *participantIdentifier) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		id.Value = strings.TrimSpace(value)
		return nil
	}

	var list []participantIdentifierItem
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	id.List = list
	return nil
}

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SABY_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("SABY_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			timeout = parsed
		}
	}

	c := &Client{
		httpClient:    &http.Client{Timeout: timeout},
		authURL:       baseURL + authPath,
		serviceURL:    baseURL + servicePath,
		accessToken:   strings.TrimSpace(os.Getenv("SABY_ACCESS_TOKEN")),
		sessionID:     strings.TrimSpace(os.Getenv("SABY_SESSION_ID")),
		login:         strings.TrimSpace(os.Getenv("SABY_LOGIN")),
		password:      strings.TrimSpace(os.Getenv("SABY_PASSWORD")),
		accountNumber: strings.TrimSpace(os.Getenv("SABY_ACCOUNT_NUMBER")),
	}
	if !c.Enabled() {
		return nil
	}
	return c
}

func (c *Client) Enabled() bool {
	if c == nil {
		return false
	}
	if c.accessToken != "" || c.sessionID != "" {
		return true
	}
	return c.login != "" && c.password != ""
}

func (c *Client) LookupParticipantID(ctx context.Context, party Party) (string, error) {
	if !c.Enabled() {
		return "", errNotConfigured
	}

	participant, err := buildParticipant(party)
	if err != nil {
		return "", err
	}

	var info participantInfo
	err = c.call(ctx, c.serviceURL, "СБИС.ИнформацияОКонтрагенте", map[string]interface{}{
		"Участник": participant,
	}, &info)
	if err != nil {
		return "", err
	}

	id := chooseParticipantID(info.Identifier)
	if id == "" {
		return "", fmt.Errorf("saby returned empty EDO participant ID")
	}
	return id, nil
}

func buildParticipant(party Party) (map[string]interface{}, error) {
	inn := digitsOnly(party.INN)
	kpp := digitsOnly(party.KPP)
	if len(inn) != 10 && len(inn) != 12 {
		return nil, fmt.Errorf("INN must contain 10 or 12 digits")
	}
	if len(inn) == 10 && len(kpp) != 9 {
		return nil, fmt.Errorf("KPP must contain 9 digits for organizations")
	}

	participant := map[string]interface{}{
		"ДопПоля": "СписокИдентификаторов",
	}
	if len(inn) == 10 {
		org := map[string]string{
			"ИНН": inn,
			"КПП": kpp,
		}
		if name := strings.TrimSpace(party.Name); name != "" {
			org["Название"] = name
		}
		participant["СвЮЛ"] = org
	} else {
		person := map[string]string{
			"ИНН": inn,
		}
		if name := strings.TrimSpace(party.Name); name != "" {
			person["Название"] = name
		}
		participant["СвФЛ"] = person
	}
	return participant, nil
}

func chooseParticipantID(identifier participantIdentifier) string {
	if id := strings.TrimSpace(identifier.Value); id != "" {
		return id
	}

	var first string
	var connected string
	for _, item := range identifier.List {
		id := strings.TrimSpace(item.ParticipantID)
		if id == "" {
			continue
		}
		if first == "" {
			first = id
		}
		if strings.EqualFold(strings.TrimSpace(item.Primary), "Да") {
			return id
		}
		if connected == "" && strings.TrimSpace(item.ConnectionState.Code) == "0" {
			connected = id
		}
	}
	if connected != "" {
		return connected
	}
	return first
}

func (c *Client) call(ctx context.Context, endpoint, method string, params interface{}, target interface{}) error {
	if c == nil {
		return errNotConfigured
	}
	if c.accessToken == "" && c.sessionID == "" {
		if err := c.authenticate(ctx); err != nil {
			return err
		}
	}
	if err := c.callOnce(ctx, endpoint, method, params, target); err != nil {
		if !isUnauthorized(err) || c.accessToken != "" || (c.login == "" || c.password == "") {
			return err
		}
		c.mu.Lock()
		c.sessionID = ""
		c.mu.Unlock()
		if authErr := c.authenticate(ctx); authErr != nil {
			return authErr
		}
		return c.callOnce(ctx, endpoint, method, params, target)
	}
	return nil
}

func (c *Client) authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessionID != "" || c.accessToken != "" {
		return nil
	}

	param := map[string]string{
		"Логин":  c.login,
		"Пароль": c.password,
	}
	if c.accountNumber != "" {
		param["НомерАккаунта"] = c.accountNumber
	}

	var sessionID string
	if err := c.callOnce(ctx, c.authURL, "СБИС.Аутентифицировать", map[string]interface{}{"Параметр": param}, &sessionID); err != nil {
		return err
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("saby authentication returned empty session ID")
	}
	c.sessionID = strings.TrimSpace(sessionID)
	return nil
}

func (c *Client) callOnce(ctx context.Context, endpoint, method string, params interface{}, target interface{}) error {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return fmt.Errorf("marshal saby request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build saby request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		req.Header.Set("X-SBISAccessToken", c.accessToken)
	} else if c.sessionID != "" {
		req.Header.Set("X-SBISSessionID", c.sessionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call saby %s: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return sabyHTTPError{statusCode: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("call saby %s failed with HTTP %d", method, resp.StatusCode)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode saby %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		if rpcResp.Error.Details != "" {
			return fmt.Errorf("saby %s failed: %s: %s", method, rpcResp.Error.Message, rpcResp.Error.Details)
		}
		return fmt.Errorf("saby %s failed: %s", method, rpcResp.Error.Message)
	}
	if target == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 {
		return fmt.Errorf("saby %s returned empty result", method)
	}
	if err := json.Unmarshal(rpcResp.Result, target); err != nil {
		return fmt.Errorf("decode saby %s result: %w", method, err)
	}
	return nil
}

type sabyHTTPError struct {
	statusCode int
}

func (e sabyHTTPError) Error() string {
	return fmt.Sprintf("saby HTTP %d", e.statusCode)
}

func isUnauthorized(err error) bool {
	var httpErr sabyHTTPError
	return errors.As(err, &httpErr) && httpErr.statusCode == http.StatusUnauthorized
}

func digitsOnly(value string) string {
	return regexp.MustCompile(`\D+`).ReplaceAllString(value, "")
}
