package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

// DailyClose는 날짜별 종가를 담습니다.
type DailyClose struct {
	DateUnix int64
	Close    float64
}

// GetChartHistory fetches daily close prices for the given symbol.
// rangeStr: "1mo", "3mo", "1y" etc.
// interval: "1d", "1wk" etc.
//
// TODO: Yahoo Finance는 비공식 API. 장기적으로 공식 제공 업체(EODHD 등)로 대체 검토 필요.
func (c *Client) GetChartHistory(ctx context.Context, symbol, rangeStr, interval string) ([]DailyClose, error) {
	endpoint := fmt.Sprintf("%s/v8/finance/chart/%s", c.baseURL, url.PathEscape(symbol))
	values := url.Values{
		"range":    []string{rangeStr},
		"interval": []string{interval},
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
		return nil, fmt.Errorf("yahoo chart history status %d for %s", resp.StatusCode, symbol)
	}
	return decodeChartHistory(raw)
}

type chartHistoryResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"chart"`
}

func decodeChartHistory(raw []byte) ([]DailyClose, error) {
	var payload chartHistoryResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode chart history: %w", err)
	}
	if len(payload.Chart.Result) == 0 {
		return nil, fmt.Errorf("chart history: no results")
	}
	result := payload.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("chart history: no quote indicators")
	}
	timestamps := result.Timestamp
	closes := result.Indicators.Quote[0].Close
	if len(timestamps) != len(closes) {
		return nil, fmt.Errorf("chart history: timestamp/close length mismatch")
	}
	out := make([]DailyClose, 0, len(timestamps))
	for i, ts := range timestamps {
		if closes[i] == 0 {
			continue // 미거래일 스킵
		}
		out = append(out, DailyClose{DateUnix: ts, Close: closes[i]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DateUnix < out[j].DateUnix })
	return out, nil
}
