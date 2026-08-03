package shippingindex

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
)

const (
	defaultHomepageURL = "https://www.indexq.org/"
)

type Service struct {
	client *auth.KIClient
}

type Quote struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	Local         string  `json:"local"`
	SourceURL     string  `json:"source_url"`
}

var (
	rowPattern     = regexp.MustCompile(`(?s)<tr class='row[12]'>\s*<td[^>]*><a href="/index/([A-Z0-9]+)\.php">([^<]+)</a></td>\s*<td[^>]*>([^<]+)</td>\s*<td[^>]*>([^<]+)</td>\s*<td[^>]*>([^<]+)</td>\s*<td[^>]*>([^<]+)</td>\s*</tr>`)
	defaultSymbols = []string{"SCFI", "BDI", "BCI", "BPI", "BSI", "BHI", "BCTI", "BDTI"}
)

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func DefaultSymbols() []string {
	out := make([]string, len(defaultSymbols))
	copy(out, defaultSymbols)
	return out
}

func (s *Service) Quotes(ctx context.Context, symbols []string) ([]Quote, error) {
	normalizedSymbols := normalizeSymbols(symbols)
	if len(normalizedSymbols) == 0 {
		normalizedSymbols = DefaultSymbols()
	}

	raw, err := s.fetchHomepage(ctx)
	if err != nil {
		return nil, err
	}

	available, err := parseHomepage(raw)
	if err != nil {
		return nil, err
	}

	quotes := make([]Quote, 0, len(normalizedSymbols))
	missing := make([]string, 0)
	for _, symbol := range normalizedSymbols {
		quote, ok := available[symbol]
		if !ok {
			missing = append(missing, symbol)
			continue
		}
		quotes = append(quotes, quote)
	}

	if len(missing) > 0 {
		return quotes, fmt.Errorf("shipping indices not found in provider response: %s", strings.Join(missing, ", "))
	}
	if len(quotes) == 0 {
		return nil, errors.New("shipping index provider returned no requested quotes")
	}

	return quotes, nil
}

func (s *Service) fetchHomepage(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, defaultHomepageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "open-trading-api shipping index resolver")

	if s.client != nil {
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("shipping index provider status %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shipping index provider status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func parseHomepage(raw []byte) (map[string]Quote, error) {
	matches := rowPattern.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		return nil, errors.New("shipping index rows not found in provider response")
	}

	quotes := make(map[string]Quote, len(matches))
	for _, match := range matches {
		if len(match) != 7 {
			continue
		}

		symbol := strings.TrimSpace(match[1])
		price, err := parseNumber(match[3])
		if err != nil {
			continue
		}
		change, err := parseNumber(match[4])
		if err != nil {
			continue
		}
		changePercent, err := parsePercent(match[5])
		if err != nil {
			continue
		}

		quotes[symbol] = Quote{
			Symbol:        symbol,
			Name:          html.UnescapeString(strings.TrimSpace(match[2])),
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Local:         strings.TrimSpace(match[6]),
			SourceURL:     defaultHomepageURL + "index/" + symbol + ".php",
		}
	}

	if len(quotes) == 0 {
		return nil, errors.New("shipping index provider response contained no parsable quotes")
	}

	return quotes, nil
}

func normalizeSymbols(symbols []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out
}

func parseNumber(value string) (float64, error) {
	value = strings.TrimSpace(html.UnescapeString(value))
	value = strings.ReplaceAll(value, ",", "")
	if value == "" || value == "-" {
		return 0, errors.New("number is empty")
	}
	return strconv.ParseFloat(value, 64)
}

func parsePercent(value string) (float64, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "%")
	return parseNumber(value)
}
