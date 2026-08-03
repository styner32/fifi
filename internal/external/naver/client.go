// Package naver provides access to Naver Finance index data.
// Uses the unofficial Naver Finance polling and sise APIs.
// These are not officially supported — may break without notice.
package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	pollingBaseURL    = "https://polling.finance.naver.com"
	siseBaseURL       = "https://finance.naver.com"
	defaultTimeout    = 15 * time.Second
)

// IndexQuote holds snapshot data for a Naver Finance index.
type IndexQuote struct {
	Code          string  // e.g. "VKOSPI", "KOSPI"
	Price         float64 // 현재가 (nv / 100)
	Change        float64 // 전일 대비 (cv / 100)
	ChangePercent float64 // 등락률 (cr)
	Open          float64 // 시가
	High          float64 // 고가
	Low           float64 // 저가
	MarketStatus  string  // "OPEN" / "CLOSE" / ""
}

// DailyClose holds a date + close price for historical data.
type DailyClose struct {
	Date  string  // "YYYY.MM.DD"
	Close float64
}

// Client is a Naver Finance HTTP client.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

func NewClient(httpClient *http.Client, userAgent string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{httpClient: httpClient, userAgent: userAgent}
}

// GetIndexQuote fetches a realtime snapshot for one Naver Finance index code.
// Known codes: "VKOSPI", "KOSPI", "KOSDAQ", "KPI200"
func (c *Client) GetIndexQuote(ctx context.Context, code string) (*IndexQuote, error) {
	url := fmt.Sprintf("%s/api/realtime?query=SERVICE_INDEX:%s", pollingBaseURL, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9")
	req.Header.Set("Referer", pollingBaseURL+"/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("naver polling request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("naver polling status %d", resp.StatusCode)
	}
	return decodePollingResponse(raw, code)
}

// GetIndexDailyHistory fetches up to n days of daily closes from Naver sise page.
// Returns rows in ascending date order (oldest first).
func (c *Client) GetIndexDailyHistory(ctx context.Context, code string, n int) ([]DailyClose, error) {
	url := fmt.Sprintf("%s/sise/sise_index_day.naver?code=%s&page=1", siseBaseURL, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", "https://finance.naver.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("naver sise request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("naver sise status %d", resp.StatusCode)
	}
	return decodeSiseHistory(raw, n)
}

// ── internal decoders ──────────────────────────────────────────────────────

type pollingResponse struct {
	ResultCode string `json:"resultCode"`
	Result     struct {
		Areas []struct {
			Name  string `json:"name"`
			Datas []struct {
				MS string  `json:"ms"` // "OPEN" / "CLOSE"
				CD string  `json:"cd"` // index code
				NV int64   `json:"nv"` // 현재가 × 100
				CV int64   `json:"cv"` // 전일 대비 × 100
				CR float64 `json:"cr"` // 등락률
				OV int64   `json:"ov"` // 시가 × 100
				HV int64   `json:"hv"` // 고가 × 100
				LV int64   `json:"lv"` // 저가 × 100
			} `json:"datas"`
		} `json:"areas"`
	} `json:"result"`
}

func decodePollingResponse(raw []byte, code string) (*IndexQuote, error) {
	var p pollingResponse
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("naver polling decode: %w", err)
	}
	if p.ResultCode != "success" {
		return nil, fmt.Errorf("naver polling resultCode: %s", p.ResultCode)
	}
	for _, area := range p.Result.Areas {
		for _, d := range area.Datas {
			if strings.EqualFold(d.CD, code) {
				return &IndexQuote{
					Code:          d.CD,
					Price:         float64(d.NV) / 100.0,
					Change:        float64(d.CV) / 100.0,
					ChangePercent: d.CR,
					Open:          float64(d.OV) / 100.0,
					High:          float64(d.HV) / 100.0,
					Low:           float64(d.LV) / 100.0,
					MarketStatus:  d.MS,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("naver polling: code %q not found in response (market may be closed)", code)
}

var siseDatePattern = regexp.MustCompile(`(\d{4}\.\d{2}\.\d{2})`)
var siseNumPattern = regexp.MustCompile(`(\d{1,3}(?:,\d{3})*\.\d+)`)

func decodeSiseHistory(raw []byte, n int) ([]DailyClose, error) {
	// Naver sise pages are EUC-KR encoded
	html := string(raw) // treat as latin-1 for regex purposes; numbers are ASCII

	// Find <tr> rows with date + numeric values
	trPattern := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	tdPattern := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	stripTags := regexp.MustCompile(`<[^>]+>`)

	var result []DailyClose
	for _, trMatch := range trPattern.FindAllString(html, -1) {
		tds := tdPattern.FindAllString(trMatch, -1)
		if len(tds) < 2 {
			continue
		}
		// First td should be a date
		firstTD := stripTags.ReplaceAllString(tds[0], "")
		firstTD = strings.TrimSpace(firstTD)
		if !siseDatePattern.MatchString(firstTD) {
			continue
		}
		date := siseDatePattern.FindString(firstTD)

		// Second td should be the closing price
		secondTD := stripTags.ReplaceAllString(tds[1], "")
		secondTD = strings.TrimSpace(secondTD)
		numStr := strings.ReplaceAll(secondTD, ",", "")
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil || val <= 0 {
			continue
		}
		result = append(result, DailyClose{Date: date, Close: val})
		if len(result) >= n {
			break
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("naver sise: no rows parsed (market may be closed or page format changed)")
	}
	// Result is newest-first from Naver; reverse to oldest-first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}
