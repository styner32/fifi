// Package kofia provides access to KOFIA FreeSIS (금융투자협회 종합통계) data.
// Uses the internal FreeSIS API at freesis.kofia.or.kr/meta/getMetaDataList.do.
// This is NOT an official API — may break without notice.
package kofia

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/fifi/internal/parse"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

// kofiaCertPEM is the required certificate for FreeSIS.
var kofiaCertPEM = []byte(`
-----BEGIN CERTIFICATE-----
MIIGbTCCBVWgAwIBAgIMdASdnGHzthmyNtWcMA0GCSqGSIb3DQEBCwUAMFAxCzAJ
BgNVBAYTAkJFMRkwFwYDVQQKExBHbG9iYWxTaWduIG52LXNhMSYwJAYDVQQDEx1H
bG9iYWxTaWduIFJTQSBPViBTU0wgQ0EgMjAxODAeFw0yNjAzMDQwOTEzMjdaFw0y
NzA0MDUwOTEzMjZaMIGAMQswCQYDVQQGEwJLUjEOMAwGA1UECBMFU2VvdWwxGDAW
BgNVBAcTD1llb25nZGV1bmdwby1ndTEvMC0GA1UEChMmS29yZWEgRmluYW5jaWFs
IEludmVzdG1lbnQgQXNzb2NpYXRpb24xFjAUBgNVBAMMDSoua29maWEub3Iua3Iw
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCq9tsT35DWDa0xVwQ1+lV0
LncpMW/5srjDoBVWObS/HaCGtXpjuZEbVu2s3nWtQx8mTpJdgvjmuT0VySoYcS/R
DG9TqXgiHTEcO4pK/i1fuDsHAXyA44DVif5F+975rWfvfStujrlCx4KBVyWdPzZj
qu5umxWM8v74C+o3fCo+3VV7sqjwNlwW1g/fTlfkDVFRSZ3LM82y6CzmUB1liLxZ
4d3qtWzj96h5BNnSjcu5i9FcMmk+unppXC7BFUZ4kVdU/yX3Rd1GlQBD2sPAh4Ge
MA6e03z/PtzbeaywpE5sBFIR4xfYhi3AplDowx2lBBtuAeMwG7jSyl7WVbFjaoBd
AgMBAAGjggMUMIIDEDAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIwADCBjgYI
KwYBBQUHAQEEgYEwfzBEBggrBgEFBQcwAoY4aHR0cDovL3NlY3VyZS5nbG9iYWxz
aWduLmNvbS9jYWNlcnQvZ3Nyc2FvdnNzbGNhMjAxOC5jcnQwNwYIKwYBBQUHMAGG
K2h0dHA6Ly9vY3NwLmdsb2JhbHNpZ24uY29tL2dzcnNhb3Zzc2xjYTIwMTgwVgYD
VR0gBE8wTTBBBgkrBgEEAaAyARQwNDAyBggrBgEFBQcCARYmaHR0cHM6Ly93d3cu
Z2xvYmFsc2lnbi5jb20vcmVwb3NpdG9yeS8wCAYGZ4EMAQICMCUGA1UdEQQeMByC
DSoua29maWEub3Iua3KCC2tvZmlhLm9yLmtyMB0GA1UdJQQWMBQGCCsGAQUFBwMB
BggrBgEFBQcDAjAfBgNVHSMEGDAWgBT473/yzXhnqN5vjySNiPGHAwKz6zAdBgNV
HQ4EFgQUSE+fINss6eS+4QhVGO79gfKNbNkwggF/BgorBgEEAdZ5AgQCBIIBbwSC
AWsBaQB2AByfaCzp+vBFaVD4G5aKh93bMhDYTObIsuOCUkrEz1mfAAABnLgfsVIA
AAQDAEcwRQIgOrOgvqAdwLbUW21xcn27nnUU4EBjAWbC3UDFSHuigP8CIQCo5nLK
wTr7ODZ9T90U9rRfXhj3XfkjVSUwvl/MD469TgB3AExj3JjlnB2riPYeij3ero+r
RKM3e1+blMP7oZz8wb4mAAABnLgfsS0AAAQDAEgwRgIhAMqoFd96OjAdwhqvvU5e
hgDHarGjC4e9iv3AGFiSg8CPAiEAtyborHpjsySQ0K/2lAFucZevDzKN8Y42aoSJ
aK1smsYAdgBgTJqven93XwHUBvySDciZ6wscffjJUhv6+hd3O5eLyQAAAZy4H7Ig
AAAEAwBHMEUCIQDWBr+Uoi3MkKtEY44sRFy2eslEZ0rE0GaNn0qeElJe6wIgMpUg
rl8Hq/P7AY2O7nEzRZM6P7LWmfKw90NCiUrokoUwDQYJKoZIhvcNAQELBQADggEB
AFt1Zj8LoIRTDY9OjVVv3rDXmRL5ybZTvaBL2JdlU8zSWK11fHRFxErYM+CRBkrf
ZN5ALwkG9Uk9CWQXbBGBix672CH6FWIZsy8eeQZap2WFeWjPXwkO/9oYxyD+alG+
jVR03Jx+KkF04pbZnTRNWZMjitWf5sr+jDo7Jo7skZL6UwGqCMTLFFg7Mj4LD8bx
buGrQcTjQcnoQ/xHqNmQbeKmssh9oPpoFaJ4FdG/ZpjCO+MAk0yhkw7n62j38l0O
nK8wV6nUYdtjkR5A7ylPkZ1vO8yWUmhh/ZlhaYRgZmac29DGtdLfOM5xlqGAAEn0
2VE6DhAbIRDeoGSaHb3l/UI=
-----END CERTIFICATE-----
`)

const (
	baseURL        = "https://freesis.kofia.or.kr"
	metaDataURL    = baseURL + "/meta/getMetaDataList.do"
	sessionURL     = baseURL + "/index.jsp"
	defaultUA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	defaultTimeout = 15 * time.Second

	// 증시자금추이 서비스 ID
	MarketFundsObjNM = "STATSCU0100000060BO"

	// Request amounts in million KRW explicitly; do not infer units from magnitude.
	marketFundsAmountUnit = "KRW_MILLION"
)

// MarketFundsRow represents a single daily row from 증시자금추이.
// All amounts are converted to 백만원 (million KRW) for consistency with KOFIA XLS display.
type MarketFundsRow struct {
	AmountUnit            string  `json:"amount_unit"`
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

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(kofiaCertPEM)

	// FreeSIS uses a certificate that may fail verification in some environments.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
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
			"tmpV40": "1000000", // 원 -> 백만원 display divisor
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

// TMPV2..6 are amounts in the requested display unit; TMPV7 is a percentage.
// Empty unit metadata is valid only for our explicit tmpV40=1000000 request.
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

	divisor := 1.0 // empty unit follows the explicitly requested million-KRW display unit
	switch strings.TrimSpace(envelope.Unit) {
	case "", "백만원", "백만 원":
	case "천원":
		divisor = 1000
	case "원":
		divisor = 1000000
	default:
		return nil, fmt.Errorf("kofia: unrecognized amount unit %q", envelope.Unit)
	}
	var rows []MarketFundsRow

	for _, item := range envelope.DS1 {
		date := str(item, "TMPV1")
		if _, e := time.Parse("20060102", date); e != nil {
			continue
		}
		for _, key := range []string{"TMPV2", "TMPV3", "TMPV4", "TMPV5", "TMPV6", "TMPV7"} {
			v, ok := parse.Float(item[key])
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return nil, fmt.Errorf("kofia: invalid or missing %s", key)
			}
		}
		rows = append(rows, MarketFundsRow{
			AmountUnit: marketFundsAmountUnit, Date: date,
			CustomerDepositMln:    numVal(item, "TMPV2") / divisor,
			DerivativesDepositMln: numVal(item, "TMPV3") / divisor,
			RPBalanceMln:          numVal(item, "TMPV4") / divisor,
			MarginReceivableMln:   numVal(item, "TMPV5") / divisor,
			ForcedSellAmountMln:   numVal(item, "TMPV6") / divisor,
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

func numVal(m map[string]any, key string) float64 { v, _ := parse.Float(m[key]); return v }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
