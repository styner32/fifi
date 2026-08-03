package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type KIClient struct {
	AppKey         string
	AppSecret      string
	BaseURL        string
	UserAgent      string
	AuthToken      string
	TokenCachePath string
	Client         *http.Client
	metricsMu      sync.Mutex
	metrics        httpCallMetrics
}

// Config represents the structure of ~/KIS/config/kis_devlp.yaml
type Config struct {
	MyApp       string `yaml:"my_app"`
	MySec       string `yaml:"my_sec"`
	PaperApp    string `yaml:"paper_app"`
	PaperSec    string `yaml:"paper_sec"`
	MyAcctStock string `yaml:"my_acct_stock"`
	PaperStock  string `yaml:"my_paper_stock"`
	MyProd      string `yaml:"my_prod"`
	MyHtsid     string `yaml:"my_htsid"`
	Prod        string `yaml:"prod"`
	VPS         string `yaml:"vps"`
	MyAgent     string `yaml:"my_agent"`
}

// TokenResponse is the JSON response from /oauth2/tokenP
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	TokenExpired string `json:"access_token_token_expired"`
}

type CachedToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	TokenExpired string `json:"access_token_token_expired"`
	SavedAt      string `json:"saved_at"`
}

func NewKIClient(appKey string, appSecret string, baseURL string, userAgent string) *KIClient {
	return &KIClient{
		AppKey:    appKey,
		AppSecret: appSecret,
		BaseURL:   baseURL,
		UserAgent: userAgent,
		Client: &http.Client{ // update it later
			Timeout: 15 * time.Second,
		},
	}
}

func (client *KIClient) SetAuthToken(authToken string) {
	client.AuthToken = authToken
}

func (client *KIClient) SetTokenCachePath(tokenCachePath string) {
	client.TokenCachePath = tokenCachePath
}

func (token CachedToken) IsValid(now time.Time) bool {
	if token.AccessToken == "" || token.TokenExpired == "" {
		return false
	}

	expiryTime, err := time.ParseInLocation("2006-01-02 15:04:05", token.TokenExpired, time.Local)
	if err != nil {
		return false
	}

	return now.Before(expiryTime.Add(-1 * time.Minute))
}

// GetAuthToken fetches a new OAuth access token
func (client *KIClient) GetAuthToken() (*TokenResponse, error) {
	return client.GetAuthTokenWithContext(context.Background())
}

// GetAuthTokenWithContext fetches a new OAuth access token with context.
func (client *KIClient) GetAuthTokenWithContext(ctx context.Context) (*TokenResponse, error) {
	url := client.BaseURL + "/oauth2/tokenP"

	payload := map[string]string{
		"grant_type": "client_credentials",
		"appkey":     client.AppKey,
		"appsecret":  client.AppSecret,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", client.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token response: %v", err)
	}

	return &tokenResponse, nil
}

// RefreshAuthToken fetches and sets a fresh OAuth access token.
func (client *KIClient) RefreshAuthToken(ctx context.Context) (*TokenResponse, error) {
	token, err := client.GetAuthTokenWithContext(ctx)
	if err != nil {
		return nil, err
	}

	client.SetAuthToken(token.AccessToken)
	if err := client.saveTokenCache(token); err != nil {
		return nil, err
	}

	log.Printf("Refreshed auth token; valid until %s", token.TokenExpired)
	return token, nil
}

func (client *KIClient) EnsureAuthToken(ctx context.Context) (*TokenResponse, error) {
	cachedToken, err := client.loadTokenCache()
	if err == nil && cachedToken.IsValid(time.Now()) {
		client.SetAuthToken(cachedToken.AccessToken)
		return &TokenResponse{
			AccessToken:  cachedToken.AccessToken,
			TokenType:    cachedToken.TokenType,
			ExpiresIn:    cachedToken.ExpiresIn,
			TokenExpired: cachedToken.TokenExpired,
		}, nil
	}

	return client.RefreshAuthToken(ctx)
}

func (client *KIClient) loadTokenCache() (*CachedToken, error) {
	if client.TokenCachePath == "" {
		return nil, fmt.Errorf("token cache path is empty")
	}

	raw, err := os.ReadFile(client.TokenCachePath)
	if err != nil {
		return nil, err
	}

	var cachedToken CachedToken
	if err := json.Unmarshal(raw, &cachedToken); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token cache: %w", err)
	}

	return &cachedToken, nil
}

func (client *KIClient) saveTokenCache(token *TokenResponse) error {
	if client.TokenCachePath == "" || token == nil {
		return nil
	}

	cacheDir := filepath.Dir(client.TokenCachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to create token cache directory: %w", err)
	}

	cachedToken := CachedToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		ExpiresIn:    token.ExpiresIn,
		TokenExpired: token.TokenExpired,
		SavedAt:      time.Now().Format(time.RFC3339),
	}

	raw, err := json.MarshalIndent(cachedToken, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token cache: %w", err)
	}

	tmpPath := client.TokenCachePath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return fmt.Errorf("failed to write token cache temp file: %w", err)
	}
	if err := os.Rename(tmpPath, client.TokenCachePath); err != nil {
		return fmt.Errorf("failed to replace token cache file: %w", err)
	}

	return nil
}
