package commodityfuture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fifi/internal/parse"
)

const (
	ProviderCME   = "cme"
	ProviderYahoo = "yahoo"

	DefaultProviderOrderCSV = "cme,yahoo"
	DefaultUserAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

	defaultTimeout        = 15 * time.Second
	defaultCMEBaseURL     = "https://www.cmegroup.com"
	defaultYahooChartBase = "https://query1.finance.yahoo.com/v8/finance/chart/"
	defaultYahooQuoteBase = "https://finance.yahoo.com/quote/"
)

var (
	errCMEBlocked          = errors.New("cme blocked request")
	errCMEMissingProductID = errors.New("cme product id is required")
)

type Config struct {
	ProviderOrder []string
	UserAgent     string
}

type Service struct {
	client        *http.Client
	providerOrder []string
	userAgent     string
}

type Instrument struct {
	Name         string `json:"name"`
	Symbol       string `json:"symbol"`
	ProductCode  string `json:"product_code"`
	CMEProductID string `json:"cme_product_id,omitempty"`
	CMEPageURL   string `json:"cme_page_url,omitempty"`
	Exchange     string `json:"exchange,omitempty"`
	Currency     string `json:"currency,omitempty"`
}

type Quote struct {
	Name          string   `json:"name"`
	Symbol        string   `json:"symbol"`
	ProductCode   string   `json:"product_code,omitempty"`
	Contract      string   `json:"contract,omitempty"`
	QuoteCode     string   `json:"quote_code,omitempty"`
	Provider      string   `json:"provider"`
	ProviderPath  string   `json:"provider_path,omitempty"`
	SourceURL     string   `json:"source_url,omitempty"`
	ReferenceURL  string   `json:"reference_url,omitempty"`
	Exchange      string   `json:"exchange,omitempty"`
	Currency      string   `json:"currency,omitempty"`
	Delay         string   `json:"delay,omitempty"`
	AsOf          string   `json:"as_of,omitempty"`
	Note          string   `json:"note,omitempty"`
	Price         *float64 `json:"price,omitempty"`
	PreviousClose *float64 `json:"previous_close,omitempty"`
	Change        *float64 `json:"change,omitempty"`
	ChangePercent *float64 `json:"change_percent,omitempty"`
	Open          *float64 `json:"open,omitempty"`
	High          *float64 `json:"high,omitempty"`
	Low           *float64 `json:"low,omitempty"`
	Volume        *float64 `json:"volume,omitempty"`
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				ShortName            string   `json:"shortName"`
				FullExchangeName     string   `json:"fullExchangeName"`
				Currency             string   `json:"currency"`
				RegularMarketTime    int64    `json:"regularMarketTime"`
				RegularMarketPrice   *float64 `json:"regularMarketPrice"`
				ChartPreviousClose   *float64 `json:"chartPreviousClose"`
				RegularMarketOpen    *float64 `json:"regularMarketOpen"`
				RegularMarketDayHigh *float64 `json:"regularMarketDayHigh"`
				RegularMarketDayLow  *float64 `json:"regularMarketDayLow"`
				RegularMarketVolume  *float64 `json:"regularMarketVolume"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

var knownInstruments = map[string]Instrument{
	"GC": {
		Name:         "Gold",
		Symbol:       "GC=F",
		ProductCode:  "GC",
		CMEProductID: "437",
		CMEPageURL:   "https://www.cmegroup.com/markets/metals/precious/gold.quotes.html",
		Exchange:     "COMEX",
		Currency:     "USD",
	},
	"HG": {
		Name:         "Copper",
		Symbol:       "HG=F",
		ProductCode:  "HG",
		CMEProductID: "438",
		CMEPageURL:   "https://www.cmegroup.com/markets/metals/base/copper.quotes.html",
		Exchange:     "COMEX",
		Currency:     "USD",
	},
	"SI": {
		Name:         "Silver",
		Symbol:       "SI=F",
		ProductCode:  "SI",
		CMEProductID: "458",
		CMEPageURL:   "https://www.cmegroup.com/markets/metals/precious/silver.quotes.html",
		Exchange:     "COMEX",
		Currency:     "USD",
	},
	"CL": {
		Name:         "WTI Crude Oil",
		Symbol:       "CL=F",
		ProductCode:  "CL",
		CMEProductID: "425",
		CMEPageURL:   "https://www.cmegroup.com/markets/energy/crude-oil/light-sweet-crude.quotes.html",
		Exchange:     "NYMEX",
		Currency:     "USD",
	},
}

func NewService(httpClient *http.Client, cfg Config) *Service {
	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	return &Service{
		client:        httpClient,
		providerOrder: normalizeProviderOrder(cfg.ProviderOrder),
		userAgent:     userAgent,
	}
}

func NormalizeProductCode(productCode string) string {
	productCode = strings.ToUpper(strings.TrimSpace(productCode))
	return strings.TrimSuffix(productCode, "=F")
}

func KnownInstrument(productCode string) (Instrument, bool) {
	productCode = NormalizeProductCode(productCode)
	instrument, ok := knownInstruments[productCode]
	return instrument, ok
}

func ResolveInstrument(name string, symbol string, productCode string) Instrument {
	productCode = NormalizeProductCode(productCode)
	if productCode == "" {
		productCode = NormalizeProductCode(symbol)
	}

	instrument, ok := KnownInstrument(productCode)
	if !ok {
		instrument = Instrument{ProductCode: productCode}
	}

	if strings.TrimSpace(name) != "" {
		instrument.Name = strings.TrimSpace(name)
	}
	if strings.TrimSpace(symbol) != "" {
		instrument.Symbol = strings.TrimSpace(symbol)
	}
	if instrument.ProductCode == "" {
		instrument.ProductCode = productCode
	}

	return instrument
}

func (s *Service) Quote(ctx context.Context, instrument Instrument) (*Quote, error) {
	instrument = ResolveInstrument(instrument.Name, instrument.Symbol, instrument.ProductCode)

	attempted := make([]string, 0, len(s.providerOrder))
	failures := make([]string, 0, len(s.providerOrder))

	for _, provider := range s.providerOrder {
		var (
			quote *Quote
			err   error
		)

		switch provider {
		case ProviderCME:
			quote, err = s.quoteFromCME(ctx, instrument)
			if errors.Is(err, errCMEMissingProductID) {
				continue
			}
		case ProviderYahoo:
			quote, err = s.quoteFromYahoo(ctx, instrument)
		default:
			continue
		}

		if err != nil {
			attempted = append(attempted, provider)
			failures = append(failures, conciseFailure(provider, err))
			continue
		}

		attempted = append(attempted, provider)
		quote.Provider = provider
		quote.ProviderPath = strings.Join(attempted, " -> ")
		if len(failures) > 0 {
			quote.Note = "fallback after " + strings.Join(failures, "; ")
		}
		if quote.ReferenceURL == "" && provider == ProviderYahoo && instrument.CMEPageURL != "" {
			quote.ReferenceURL = instrument.CMEPageURL
		}
		return quote, nil
	}

	if len(failures) == 0 {
		return nil, errors.New("no commodity future provider succeeded")
	}
	return nil, errors.New(strings.Join(failures, "; "))
}

func (s *Service) quoteFromCME(ctx context.Context, instrument Instrument) (*Quote, error) {
	productID := strings.TrimSpace(instrument.CMEProductID)
	if productID == "" {
		return nil, errCMEMissingProductID
	}

	endpoint := defaultCMEBaseURL + "/CmeWS/mvc/quotes/v2/" + productID
	raw, err := s.get(ctx, endpoint, "application/json, text/plain, */*", instrument.CMEPageURL, true)
	if err != nil {
		return nil, err
	}

	var blocked struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &blocked); err == nil {
		message := strings.TrimSpace(blocked.Message)
		if strings.Contains(strings.ToLower(message), "suspected web scraping") {
			return nil, errCMEBlocked
		}
	}

	var payload struct {
		Message    string           `json:"message"`
		QuoteDelay any              `json:"quoteDelay"`
		Quotes     []map[string]any `json:"quotes"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("cme decode failed: %w", err)
	}
	if strings.TrimSpace(payload.Message) != "" && len(payload.Quotes) == 0 {
		return nil, errors.New(strings.TrimSpace(payload.Message))
	}

	row := selectCMEQuote(payload.Quotes)
	if row == nil {
		return nil, errors.New("cme response missing quotes")
	}

	quote := &Quote{
		Name:        firstNonEmptyString(instrument.Name, stringValue(row["productName"]), instrument.ProductCode),
		Symbol:      firstNonEmptyString(instrument.Symbol, instrument.ProductCode),
		ProductCode: firstNonEmptyString(instrument.ProductCode, NormalizeProductCode(stringValue(row["productCode"]))),
		Contract:    firstNonEmptyString(stringValue(row["expirationMonth"]), stringValue(row["productName"]), instrument.Name),
		QuoteCode:   firstNonEmptyString(stringValue(row["quoteCode"]), stringValue(row["code"])),
		SourceURL:   firstNonEmptyString(instrument.CMEPageURL, endpoint),
		Exchange:    firstNonEmptyString(stringValue(row["exchangeCode"]), instrument.Exchange),
		Currency:    firstNonEmptyString(stringValue(row["currency"]), instrument.Currency),
		Delay:       formatDelay(payload.QuoteDelay),
		AsOf:        stringValue(row["updated"]),
	}

	quote.Price = optionalFloatValue(row["last"])
	quote.PreviousClose = optionalFloatValue(row["priorSettle"])
	quote.Change = optionalFloatValue(row["change"])
	quote.ChangePercent = optionalPercentValue(row["percentageChange"])
	quote.Open = optionalFloatValue(row["open"])
	quote.High = optionalFloatValue(row["high"])
	quote.Low = optionalFloatValue(row["low"])
	quote.Volume = optionalFloatValue(row["volume"])

	if quote.Change == nil && quote.Price != nil && quote.PreviousClose != nil {
		quote.Change = floatPointer(*quote.Price - *quote.PreviousClose)
	}
	if quote.ChangePercent == nil && quote.Change != nil && quote.PreviousClose != nil && *quote.PreviousClose != 0 {
		quote.ChangePercent = floatPointer(*quote.Change / *quote.PreviousClose * 100)
	}

	return quote, nil
}

func (s *Service) quoteFromYahoo(ctx context.Context, instrument Instrument) (*Quote, error) {
	symbol := strings.TrimSpace(instrument.Symbol)
	if symbol == "" {
		return nil, errors.New("yahoo symbol is required")
	}

	endpoint := defaultYahooChartBase + symbol + "?interval=1d&range=5d"
	raw, err := s.get(ctx, endpoint, "application/json, text/plain, */*", "", false)
	if err != nil {
		return nil, err
	}

	var payload yahooChartResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("yahoo decode failed: %w", err)
	}
	if payload.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo chart error: %s", strings.TrimSpace(payload.Chart.Error.Description))
	}
	if len(payload.Chart.Result) == 0 {
		return nil, errors.New("yahoo chart result missing")
	}

	result := payload.Chart.Result[0]
	var latestQuote *struct {
		Open   []*float64 `json:"open"`
		High   []*float64 `json:"high"`
		Low    []*float64 `json:"low"`
		Close  []*float64 `json:"close"`
		Volume []*float64 `json:"volume"`
	}
	if len(result.Indicators.Quote) > 0 {
		latestQuote = &result.Indicators.Quote[0]
	}

	price := result.Meta.RegularMarketPrice
	if price == nil && latestQuote != nil {
		price = lastNonNil(latestQuote.Close)
	}
	previousClose := result.Meta.ChartPreviousClose
	if previousClose == nil && latestQuote != nil {
		previousClose = previousCloseFromSeries(latestQuote.Close)
	}

	quote := &Quote{
		Name:          firstNonEmptyString(result.Meta.ShortName, instrument.Name, instrument.ProductCode),
		Symbol:        symbol,
		ProductCode:   firstNonEmptyString(instrument.ProductCode, NormalizeProductCode(symbol)),
		Contract:      firstNonEmptyString(result.Meta.ShortName, instrument.Name),
		SourceURL:     defaultYahooQuoteBase + url.PathEscape(symbol),
		ReferenceURL:  instrument.CMEPageURL,
		Exchange:      firstNonEmptyString(result.Meta.FullExchangeName, instrument.Exchange),
		Currency:      firstNonEmptyString(result.Meta.Currency, instrument.Currency),
		AsOf:          formatUnix(result.Meta.RegularMarketTime),
		Price:         price,
		PreviousClose: previousClose,
	}

	if latestQuote != nil {
		quote.Open = firstNonNil(result.Meta.RegularMarketOpen, lastNonNil(latestQuote.Open))
		quote.High = firstNonNil(result.Meta.RegularMarketDayHigh, lastNonNil(latestQuote.High))
		quote.Low = firstNonNil(result.Meta.RegularMarketDayLow, lastNonNil(latestQuote.Low))
		quote.Volume = firstNonNil(result.Meta.RegularMarketVolume, lastNonNil(latestQuote.Volume))
	} else {
		quote.Open = result.Meta.RegularMarketOpen
		quote.High = result.Meta.RegularMarketDayHigh
		quote.Low = result.Meta.RegularMarketDayLow
		quote.Volume = result.Meta.RegularMarketVolume
	}

	if quote.Price != nil && quote.PreviousClose != nil {
		change := *quote.Price - *quote.PreviousClose
		quote.Change = floatPointer(change)
		if *quote.PreviousClose != 0 {
			quote.ChangePercent = floatPointer(change / *quote.PreviousClose * 100)
		}
	}

	return quote, nil
}

func (s *Service) get(ctx context.Context, endpoint string, accept string, referer string, ajax bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", s.userAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if ajax {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	return raw, nil
}

func (s *Service) httpClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	return &http.Client{Timeout: defaultTimeout}
}

func normalizeProviderOrder(order []string) []string {
	if len(order) == 0 {
		order = strings.Split(DefaultProviderOrderCSV, ",")
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(order))
	for _, provider := range order {
		provider = strings.ToLower(strings.TrimSpace(provider))
		switch provider {
		case ProviderCME, ProviderYahoo:
		default:
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		out = append(out, provider)
	}
	if len(out) == 0 {
		return []string{ProviderCME, ProviderYahoo}
	}
	return out
}

func conciseFailure(provider string, err error) string {
	if errors.Is(err, errCMEBlocked) {
		return provider + ": blocked"
	}
	return provider + ": " + err.Error()
}

func selectCMEQuote(quotes []map[string]any) map[string]any {
	for _, quote := range quotes {
		if boolValue(quote["isFrontMonth"]) {
			return quote
		}
	}
	if len(quotes) == 0 {
		return nil
	}
	return quotes[0]
}

func optionalFloatValue(value any) *float64 {
	if str, ok := value.(string); ok {
		clean := strings.TrimSpace(strings.ReplaceAll(str, ",", ""))
		if clean == "" || clean == "-" {
			return nil
		}
		if strings.EqualFold(clean, "UNCH") {
			return floatPointer(0)
		}
		if strings.HasSuffix(clean, "%") {
			clean = strings.TrimSuffix(clean, "%")
		}
		value = clean
	}
	if parsed, ok := parse.Float(value); ok {
		return floatPointer(parsed)
	}
	return nil
}

func optionalPercentValue(value any) *float64 {
	return optionalFloatValue(value)
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		typed = strings.ToLower(strings.TrimSpace(typed))
		return typed == "true" || typed == "1" || typed == "y"
	default:
		return false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatDelay(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case float64:
		return humanizeSeconds(int(typed))
	case int:
		return humanizeSeconds(typed)
	case int64:
		return humanizeSeconds(int(typed))
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return ""
		}
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return humanizeSeconds(parsed)
		}
		return typed
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func humanizeSeconds(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d min", seconds/60)
	}
	return fmt.Sprintf("%d sec", seconds)
}

func formatUnix(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func floatPointer(value float64) *float64 {
	v := value
	return &v
}

func firstNonNil(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func lastNonNil(values []*float64) *float64 {
	for i := len(values) - 1; i >= 0; i-- {
		if values[i] != nil {
			return values[i]
		}
	}
	return nil
}

func previousCloseFromSeries(values []*float64) *float64 {
	foundLatest := false
	for i := len(values) - 1; i >= 0; i-- {
		if values[i] == nil {
			continue
		}
		if !foundLatest {
			foundLatest = true
			continue
		}
		return values[i]
	}
	return nil
}
