package yahoo

import (
	"encoding/json"
	"fmt"
	"strings"
)

type chartResponse struct {
	Chart struct {
		Result []chartResult `json:"result"`
		Error  any           `json:"error"`
	} `json:"chart"`
}

type chartResult struct {
	Meta chartMeta `json:"meta"`
}

type chartMeta struct {
	Symbol             string  `json:"symbol"`
	Currency           string  `json:"currency"`
	RegularMarketPrice float64 `json:"regularMarketPrice"`
	ChartPreviousClose float64 `json:"chartPreviousClose"`
	RegularMarketTime  int64   `json:"regularMarketTime"`
	ShortName          string  `json:"shortName"`
	LongName           string  `json:"longName"`
}

func decodeChartQuote(raw []byte) (*Quote, error) {
	var payload chartResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode yahoo chart response: %w", err)
	}

	if len(payload.Chart.Result) == 0 {
		return nil, fmt.Errorf("yahoo chart response contained no results")
	}

	meta := payload.Chart.Result[0].Meta
	symbol := strings.TrimSpace(meta.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("yahoo chart response missing symbol")
	}
	name := strings.TrimSpace(meta.ShortName)
	if name == "" {
		name = strings.TrimSpace(meta.LongName)
	}
	
	changePercent := 0.0
	if meta.ChartPreviousClose != 0 {
		changePercent = (meta.RegularMarketPrice - meta.ChartPreviousClose) / meta.ChartPreviousClose * 100.0
	}

	return &Quote{
		Symbol:         symbol,
		Name:           name,
		Price:          meta.RegularMarketPrice,
		PreviousClose:  meta.ChartPreviousClose,
		ChangePercent:  changePercent,
		Currency:       strings.TrimSpace(meta.Currency),
		MarketTimeUnix: meta.RegularMarketTime,
	}, nil
}

func missingSymbolsError(symbols []string, quotes map[string]Quote) error {
	missing := make([]string, 0)
	for _, symbol := range symbols {
		if _, ok := quotes[symbol]; !ok {
			missing = append(missing, symbol)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("yahoo quote missing: %s", strings.Join(missing, ", "))
}

func normalizeSymbols(symbols []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
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
