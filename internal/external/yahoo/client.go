package yahoo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultBaseURL   = "https://query1.finance.yahoo.com"
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	defaultTimeout   = 15 * time.Second
)

type Config struct {
	BaseURL   string
	UserAgent string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

type Quote struct {
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name,omitempty"`
	Price          float64 `json:"price"`
	PreviousClose  float64 `json:"previous_close"`
	ChangePercent  float64 `json:"change_percent"`
	Currency       string  `json:"currency,omitempty"`
	MarketTimeUnix int64   `json:"market_time_unix,omitempty"`
}

func NewClient(httpClient *http.Client, cfg Config) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{httpClient: httpClient, baseURL: baseURL, userAgent: userAgent}
}

func (c *Client) GetQuotes(ctx context.Context, symbols []string) (map[string]Quote, error) {
	cleanSymbols := normalizeSymbols(symbols)
	if len(cleanSymbols) == 0 {
		return nil, fmt.Errorf("symbols are required")
	}

	g, ctx := errgroup.WithContext(ctx)
	
	var mu sync.Mutex
	quotes := make(map[string]Quote, len(cleanSymbols))
	
	for _, sym := range cleanSymbols {
		symbol := sym // capture loop variable
		g.Go(func() error {
			q, err := c.getSingleChartQuote(ctx, symbol)
			if err != nil {
				// We don't fail the whole group for one missing symbol, just return nil.
				// The missingSymbolsError will handle it later.
				return nil
			}
			
			mu.Lock()
			quotes[symbol] = *q
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if len(quotes) == 0 {
		return nil, fmt.Errorf("yahoo quote response contained no valid quotes")
	}

	return quotes, missingSymbolsError(cleanSymbols, quotes)
}

func (c *Client) getSingleChartQuote(ctx context.Context, symbol string) (*Quote, error) {
	endpoint := fmt.Sprintf("%s/v8/finance/chart/%s", c.baseURL, url.PathEscape(symbol))
	values := url.Values{
		"range":    []string{"1d"},
		"interval": []string{"1d"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo chart status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	return decodeChartQuote(raw)
}
