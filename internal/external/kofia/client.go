// Package kofia provides access to KOFIA FreeSIS (금융투자협회 종합통계) data.
// Uses the internal FreeSIS API at freesis.kofia.or.kr/meta/getMetaDataList.do.
// This is NOT an official API — may break without notice.
package kofia

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

const (
	baseURL        = "https://freesis.kofia.or.kr"
	metaDataURL    = baseURL + "/meta/getMetaDataList.do"
	sessionURL     = baseURL + "/index.jsp"
	defaultUA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	defaultTimeout = 15 * time.Second

	// 증시자금추이 서비스 ID
	MarketFundsObjNM = "STATSCU0100000060BO"

	// API 원시값은 천원(KRW) 단위. 억원으로 변환하려면 ÷1억 (100,000,000).
	// 백만원으로 변환하려면 ÷1,000.
	wonToMillionKRW = 1_000.0
)

// MarketFundsRow represents a single daily row from 증시자금추이.
// All amounts are converted to 백만원 (million KRW) for consistency with KOFIA XLS display.
type MarketFundsRow struct {
	Date                  string  `json:"date"`                    // YYYYMMDD
	CustomerDepositMln    float64 `json:"customer_deposit_mln"`    // 투자자예탁금 (파생제외)
	DerivativesDepositMln float64 `json:"derivatives_deposit_mln"` // 장내파생 거래 예수금
	RPBalanceMln          float64 `json:"rp_balance_mln"`          // RP 매도잔고
	MarginReceivableMln   float64 `json:"margin_receivable_mln"`   // 위탁매매 미수금
	ForcedSellAmountMln   float64 `json:"forced_sell_amount_mln"`  // 실제 반대매매금액
	ForcedSellRatioPct    float64 `json:"forced_sell_ratio_pct"`   // 미수금 대비 반대매매비중(%)
}

// Client is a KOFIA FreeSIS HTTP client with session management.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a KOFIA client with a cookie jar for session management.
func NewClient(userAgent string) *Client {
	jar, _ := cookiejar.New(nil)
	if userAgent == "" {
		userAgent = defaultUA
	}
	// FreeSIS uses a certificate that may fail verification in some environments.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout, Jar: jar, Transport: transport},
		userAgent:  userAgent,
	}
}

// GetMarketFunds fetches 증시자금추이 data for the given date range.
// startDate and endDate are in YYYYMMDD format.
func (c *Client) GetMarketFunds(ctx context.Context, startDate, endDate string) ([]MarketFundsRow, error) {
	// No explicit session needed — the cookie jar handles it.
	payload := map[string]any{
		"dmSearch": map[string]any{
			"tmpV40": "100",
			"tmpV41": "1",
			"tmpV1":  "D",
			"tmpV45": startDate,
			"tmpV46": endDate,
			"OBJ_NM": MarketFundsObjNM,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metaDataURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kofia request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kofia status %d: %s", resp.StatusCode, string(raw[:min(len(raw), 200)]))
	}
	return decodeMarketFunds(raw)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("Referer", baseURL+"/stat/FreeSIS.do?parentDivId=MSIS10000000000000&serviceId=STATSCU0100000060")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
}

// FreeSIS API response structure:
//
//	{
//	  "unit": "",
//	  "ds1": [
//	    {
//	      "TMPV1": "20260604",           // date
//	      "TMPV2": 13969476166975,        // 투자자예탁금 (천원)
//	      "TMPV3": 5351851970083,         // 파생 예수금 (천원)
//	      "TMPV4": 11519123968170,        // RP 매도잔고 (천원)
//	      "TMPV5": 182925928283,          // 위탁매매 미수금 (천원)
//	      "TMPV6": 2431103358,            // 반대매매금액 (천원)
//	      "TMPV7": 1.8                    // 반대매매비중(%)
//	    }
//	  ]
//	}
func decodeMarketFunds(raw []byte) ([]MarketFundsRow, error) {
	var envelope struct {
		Unit string           `json:"unit"`
		DS1  []map[string]any `json:"ds1"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("kofia decode: %w", err)
	}
	if len(envelope.DS1) == 0 {
		return nil, fmt.Errorf("kofia: empty ds1 result")
	}

	var rows []MarketFundsRow
	for _, item := range envelope.DS1 {
		date := str(item, "TMPV1")
		if date == "" {
			continue
		}
		rows = append(rows, MarketFundsRow{
			Date:                  date,
			CustomerDepositMln:    numVal(item, "TMPV2") / wonToMillionKRW,
			DerivativesDepositMln: numVal(item, "TMPV3") / wonToMillionKRW,
			RPBalanceMln:          numVal(item, "TMPV4") / wonToMillionKRW,
			MarginReceivableMln:   numVal(item, "TMPV5") / wonToMillionKRW,
			ForcedSellAmountMln:   numVal(item, "TMPV6") / wonToMillionKRW,
			ForcedSellRatioPct:    numVal(item, "TMPV7"),
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("kofia: no valid rows parsed")
	}
	return rows, nil
}

func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func numVal(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
