package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	expiredTokenMessageCode  = "EGW00123" // 토큰 만료
	rateLimitMessageCode     = "EGW00201" // 초당 거래건수 초과
	rateLimitRetryDelay      = 3 * time.Second
	rateLimitMaxRetries      = 3
)

type RESTResponse struct {
	StatusCode    int
	Headers       http.Header
	Body          map[string]any
	RawBody       []byte
	ParseError    string
	RequestMethod string
	RequestURL    string
}

func (r *RESTResponse) IsOK() bool {
	if r == nil {
		return false
	}

	rtCode, _ := r.Body["rt_cd"].(string)
	return rtCode == "0"
}

func (r *RESTResponse) MessageCode() string {
	if r == nil {
		return ""
	}

	msgCode, _ := r.Body["msg_cd"].(string)
	return msgCode
}

func (r *RESTResponse) Message() string {
	if r == nil {
		return ""
	}

	msg, _ := r.Body["msg1"].(string)
	return msg
}

func (client *KIClient) Get(
	ctx context.Context,
	apiPath string,
	trID string,
	trCont string,
	params map[string]string,
) (*RESTResponse, error) {
	return client.do(ctx, http.MethodGet, apiPath, trID, trCont, params, nil, nil)
}

func (client *KIClient) Post(
	ctx context.Context,
	apiPath string,
	trID string,
	trCont string,
	payload any,
) (*RESTResponse, error) {
	return client.do(ctx, http.MethodPost, apiPath, trID, trCont, nil, payload, nil)
}

func (client *KIClient) do(
	ctx context.Context,
	method string,
	apiPath string,
	trID string,
	trCont string,
	params map[string]string,
	payload any,
	extraHeaders map[string]string,
) (*RESTResponse, error) {
	return client.doWithRetry(ctx, method, apiPath, trID, trCont, params, payload, extraHeaders, true, 0)
}

func (client *KIClient) doWithRetry(
	ctx context.Context,
	method string,
	apiPath string,
	trID string,
	trCont string,
	params map[string]string,
	payload any,
	extraHeaders map[string]string,
	allowRefresh bool,
	rateLimitRetry int,
) (*RESTResponse, error) {
	if client.AuthToken == "" {
		return nil, fmt.Errorf("auth token is empty; call GetAuthToken() and SetAuthToken() first")
	}

	endpoint := strings.TrimRight(client.BaseURL, "/") + apiPath
	if len(params) > 0 {
		values := url.Values{}
		for key, value := range params {
			values.Set(key, value)
		}
		endpoint += "?" + values.Encode()
	}

	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range client.baseHeaders(trID, trCont, extraHeaders) {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := &RESTResponse{
		StatusCode:    resp.StatusCode,
		Headers:       resp.Header.Clone(),
		RawBody:       rawBody,
		RequestMethod: req.Method,
		RequestURL:    req.URL.String(),
	}

	var parsed map[string]any
	if len(bytes.TrimSpace(rawBody)) == 0 {
		response.ParseError = "empty response body"
		return response, fmt.Errorf("empty response body")
	}

	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		response.ParseError = err.Error()
		return response, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	response.Body = parsed

	if response.IsOK() {
		return response, nil
	}

	if allowRefresh && response.MessageCode() == expiredTokenMessageCode {
		if _, err := client.RefreshAuthToken(ctx); err != nil {
			return response, fmt.Errorf("token expired and refresh failed: %w", err)
		}
		return client.doWithRetry(ctx, method, apiPath, trID, trCont, params, payload, extraHeaders, false, rateLimitRetry)
	}

	if response.MessageCode() == rateLimitMessageCode && rateLimitRetry < rateLimitMaxRetries {
		select {
		case <-ctx.Done():
			return response, ctx.Err()
		case <-time.After(rateLimitRetryDelay):
		}
		return client.doWithRetry(ctx, method, apiPath, trID, trCont, params, payload, extraHeaders, allowRefresh, rateLimitRetry+1)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(rawBody))
	}

	return response, fmt.Errorf(
		"API business error: rt_cd=%v msg_cd=%s msg1=%s",
		response.Body["rt_cd"],
		response.MessageCode(),
		response.Message(),
	)
}

func (client *KIClient) baseHeaders(trID string, trCont string, extra map[string]string) map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "text/plain",
		"charset":       "UTF-8",
		"authorization": "Bearer " + client.AuthToken,
		"appkey":        client.AppKey,
		"appsecret":     client.AppSecret,
		"custtype":      "P",
	}

	if client.UserAgent != "" {
		headers["User-Agent"] = client.UserAgent
	}
	if trID != "" {
		headers["tr_id"] = trID
	}
	if trCont != "" {
		headers["tr_cont"] = trCont
	}

	for key, value := range extra {
		headers[key] = value
	}

	return headers
}
