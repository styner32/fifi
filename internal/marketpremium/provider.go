package marketpremium

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderEnvKey      = "DCF_MARKET_PREMIUM_PROVIDER"
	FileEnvKey          = "DCF_MARKET_PREMIUM_FILE"
	URLEnvKey           = "DCF_MARKET_PREMIUM_URL"
	defaultProvider     = "damodaran"
	defaultDamodaranURL = "https://pages.stern.nyu.edu/adamodar/New_Home_Page/home.htm"
)

type Value struct {
	Rate     float64 `json:"rate"`
	Provider string  `json:"provider"`
	Source   string  `json:"source"`
	Note     string  `json:"note,omitempty"`
	AsOf     string  `json:"as_of,omitempty"`
	URL      string  `json:"url,omitempty"`
}

type Provider interface {
	Resolve(ctx context.Context) (*Value, error)
}

type Resolver struct {
	HTTPClient *http.Client
}

type FileProvider struct {
	Path string
}

type DamodaranProvider struct {
	HTTPClient *http.Client
	URL        string
}

type fileValue struct {
	MarketPremium float64 `json:"market_premium"`
	Rate          float64 `json:"rate"`
	AsOf          string  `json:"as_of"`
	Source        string  `json:"source"`
	Note          string  `json:"note"`
}

var damodaranCurrentPattern = regexp.MustCompile(`Implied ERP on\s+([A-Za-z]+\s+\d{1,2},\s+\d{4})\s*=\s*([0-9]+(?:\.[0-9]+)?)%`)

func (r Resolver) Resolve(ctx context.Context) (*Value, error) {
	providerName := strings.ToLower(strings.TrimSpace(os.Getenv(ProviderEnvKey)))
	if providerName == "" || providerName == "auto" {
		providerName = defaultProvider
	}

	switch providerName {
	case "damodaran":
		provider := DamodaranProvider{
			HTTPClient: r.httpClient(),
			URL:        strings.TrimSpace(os.Getenv(URLEnvKey)),
		}
		if provider.URL == "" {
			provider.URL = defaultDamodaranURL
		}
		return provider.Resolve(ctx)
	case "file":
		path := strings.TrimSpace(os.Getenv(FileEnvKey))
		if path == "" {
			return nil, fmt.Errorf("%s is required when %s=file", FileEnvKey, ProviderEnvKey)
		}
		return FileProvider{Path: path}.Resolve(ctx)
	default:
		return nil, fmt.Errorf("unsupported market premium provider %q", providerName)
	}
}

func (p FileProvider) Resolve(context.Context) (*Value, error) {
	raw, err := os.ReadFile(strings.TrimSpace(p.Path))
	if err != nil {
		return nil, err
	}

	var payload fileValue
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode market premium file: %w", err)
	}

	rate := payload.MarketPremium
	if rate == 0 {
		rate = payload.Rate
	}
	if rate <= 0 {
		return nil, fmt.Errorf("market premium missing in %s", p.Path)
	}
	if rate > 1 {
		rate /= 100
	}

	source := payload.Source
	if source == "" {
		source = p.Path
	}

	return &Value{
		Rate:     rate,
		Provider: "file",
		Source:   source,
		Note:     payload.Note,
		AsOf:     payload.AsOf,
	}, nil
}

func (p DamodaranProvider) Resolve(ctx context.Context) (*Value, error) {
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	url := strings.TrimSpace(p.URL)
	if url == "" {
		url = defaultDamodaranURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "open-trading-api market premium resolver")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	body := string(raw)
	matches := damodaranCurrentPattern.FindStringSubmatch(body)
	if len(matches) != 3 {
		return nil, fmt.Errorf("implied ERP pattern not found in provider response")
	}

	rate, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse provider rate: %w", err)
	}

	return &Value{
		Rate:     rate / 100,
		Provider: "damodaran",
		Source:   "Damodaran implied ERP",
		Note:     "US implied equity risk premium from current update",
		AsOf:     matches[1],
		URL:      url,
	}, nil
}

func (r Resolver) httpClient() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}
