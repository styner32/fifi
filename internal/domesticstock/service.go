package domesticstock

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/envcfg"
	"github.com/fifi/internal/parse"
)

const (
	marketTimePath                = "/uapi/domestic-stock/v1/quotations/market-time"
	inquirePricePath              = "/uapi/domestic-stock/v1/quotations/inquire-price"
	inquireIndexPricePath         = "/uapi/domestic-stock/v1/quotations/inquire-index-price"
	programTradeTodayPath         = "/uapi/domestic-stock/v1/quotations/comp-program-trade-today"
	viStatusPath                  = "/uapi/domestic-stock/v1/quotations/inquire-vi-status"
	marketFundsPath               = "/uapi/domestic-stock/v1/quotations/mktfunds"
	dailyItemChartPricePath       = "/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice"
	creditBalancePath             = "/uapi/domestic-stock/v1/ranking/credit-balance"
	dailyCreditBalancePath        = "/uapi/domestic-stock/v1/quotations/daily-credit-balance"
	investorProgramTradeTodayPath = "/uapi/domestic-stock/v1/quotations/investor-program-trade-today"
	investorTimeByMarketPath      = "/uapi/domestic-stock/v1/quotations/inquire-investor-time-by-market"
	indexMasterDownloadURL        = "https://new.real.download.dws.co.kr/common/master/idxcode.mst.zip"
	kospiMasterDownloadURL        = "https://new.real.download.dws.co.kr/common/master/kospi_code.mst.zip"
	kospiMasterFilename           = "kospi_code.mst"
	vkospiCodeEnvKey              = "VKOSPI_INDEX_CODE"
	kospiMasterCacheEnvKey        = "KOSPI_MASTER_CACHE_FILE"
	kospiActualPBRCacheEnvKey     = "KOSPI_ACTUAL_PBR_CACHE_FILE"
	kospiActualPBRRPMEnvKey       = "KOSPI_ACTUAL_PBR_RATE_LIMIT_RPM"
	kospiActualPBRDebugEnvKey     = "KOSPI_ACTUAL_PBR_DEBUG"
	kospiActualPBRProgressEnvKey  = "KOSPI_ACTUAL_PBR_PROGRESS"
	kospiProxyScaleEnvKey         = "KOSPI_PROXY_PBR_EARNINGS_SCALE"
	defaultProgramMarketClass     = "K"
	defaultKOSPIMasterCache       = ".cache/kospi_code.mst"
	defaultKOSPIActualPBRCache    = ".cache/kospi_actual_pbr.json"
)

const (
	marketTimeTRID                = "HHMCM000002C0"
	inquirePriceTRID              = "FHKST01010100"
	inquireIndexPriceTRID         = "FHPUP02100000"
	programTradeTodayTRID         = "FHPPG04600101"
	viStatusTRID                  = "FHPST01390000"
	marketFundsTRID               = "FHKST649100C0"
	dailyItemChartPriceTRID       = "FHKST03010100"
	creditBalanceTRID             = "FHKST17010000"
	dailyCreditBalanceTRID        = "FHKST130400C0"
	investorProgramTradeTodayTRID = "HHPPG046600C1"
	investorTimeByMarketTRID      = "FHPTJ04030000"
)

var (
	defaultVKOSPICandidates = []string{"0503", "2050"} // 0503: idxcode.mst에서 확인된 VKOSPI 코드
	indexCodePattern        = regexp.MustCompile(`^\s*(\d{5})`)
	kospiPart2Width         = 227
	kospiPart2FieldWidths   = []int{
		2, 1, 4, 4, 4,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 9, 5, 5, 1,
		1, 1, 2, 1, 1,
		1, 2, 2, 2, 3,
		1, 3, 12, 12, 8,
		15, 21, 2, 7, 1,
		1, 1, 1, 1, 9,
		9, 9, 5, 9, 8,
		9, 3, 1, 1, 1,
	}
)

const defaultKOSPIActualPBRRPM = 18

type Service struct {
	client            *auth.KIClient
	vkospiMu          sync.RWMutex
	resolvedVKOSPICode string
}

type RSIResult struct {
	Symbol     string
	Period     int
	Last       float64
	Signal     string
	SampleSize int
}

type ProxyPBRConstituent struct {
	Code       string
	Name       string
	MarketCap  float64
	ROE        float64
	NetIncome  float64
	BookEquity float64
	ProxyPBR   float64
	Coverage   float64
	BaseDate   string
}

type ProxyPBRResult struct {
	Market             string
	Method             string
	TargetCoverage     float64
	RawCoverage        float64
	UsedCoverage       float64
	SelectedCount      int
	UsedCount          int
	SkippedCount       int
	TotalCount         int
	TotalMarketCap     float64
	RawSelectedCap     float64
	UsedMarketCap      float64
	SkippedMarketCap   float64
	AggregateBookValue float64
	ProxyPBR           float64
	EarningsScale      float64
	CalibrationSymbol  string
	BasisDate          string
	Constituents       []ProxyPBRConstituent
}

type ActualPBRConstituent struct {
	Code       string
	Name       string
	MarketCap  float64
	PBR        float64
	BookEquity float64
	Coverage   float64
	BaseDate   string
	CacheHit   bool
}

type ActualPBRResult struct {
	Market             string
	Method             string
	TargetCoverage     float64
	RawCoverage        float64
	UsedCoverage       float64
	SelectedCount      int
	UsedCount          int
	SkippedCount       int
	TotalCount         int
	CacheHitCount      int
	APICallCount       int
	TotalMarketCap     float64
	RawSelectedCap     float64
	UsedMarketCap      float64
	SkippedMarketCap   float64
	AggregateBookValue float64
	WeightedPBR        float64
	BusinessDate       string
	ActualPBRCachePath string
	MasterCachePath    string
	MasterJSONPath     string
	MasterLoadTime     time.Duration
	CacheLoadTime      time.Duration
	RateLimitWaitTime  time.Duration
	PriceFetchTime     time.Duration
	CacheSaveTime      time.Duration
	TotalDuration      time.Duration
	Constituents       []ActualPBRConstituent
}

type kospiMasterRecord struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	MarketCap float64 `json:"market_cap"`
	NetIncome float64 `json:"net_income"`
	ROE       float64 `json:"roe"`
	BaseDate  string  `json:"base_date"`
}

type kospiMasterJSONField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type kospiMasterJSONCache struct {
	BusinessDate string                 `json:"business_date"`
	GeneratedAt  string                 `json:"generated_at"`
	SourcePath   string                 `json:"source_path"`
	RecordCount  int                    `json:"record_count"`
	Fields       []kospiMasterJSONField `json:"fields"`
	Records      []kospiMasterRecord    `json:"records"`
}

type actualPBRCacheEntry struct {
	PBR          float64 `json:"pbr"`
	MarketCap    float64 `json:"market_cap"`
	BusinessDate string  `json:"business_date"`
	UpdatedAt    string  `json:"updated_at"`
	Name         string  `json:"name,omitempty"`
}

type actualPBRCache map[string]actualPBRCacheEntry

type actualPBRLookupResult struct {
	PBR           float64
	MarketCap     float64
	CacheHit      bool
	WaitDuration  time.Duration
	FetchDuration time.Duration
	CacheSaveTime time.Duration
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func discoverVKOSPICodeFromMaster(ctx context.Context, client *auth.KIClient) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexMasterDownloadURL, nil)
	if err != nil {
		return "", err
	}

	var resp *http.Response
	if client != nil {
		resp, err = client.Do(req)
	} else {
		resp, err = http.DefaultClient.Do(req)
	}
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("master download failed (status %d)", resp.StatusCode)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	readerAt := bytes.NewReader(zipBytes)
	zipReader, err := zip.NewReader(readerAt, int64(len(zipBytes)))
	if err != nil {
		return "", err
	}

	for _, file := range zipReader.File {
		if strings.EqualFold(file.Name, "idxcode.mst") {
			fileReader, openErr := file.Open()
			if openErr != nil {
				return "", openErr
			}
			defer fileReader.Close()

			scanner := bufio.NewScanner(fileReader)
			for scanner.Scan() {
				line := scanner.Text()
				upperLine := strings.ToUpper(line)
				if !strings.Contains(upperLine, "VKOSPI") && !strings.Contains(upperLine, "V-KOSPI") {
					continue
				}
				code := extractIndexCode(line)
				if code != "" {
					return code, nil
				}
			}

			if err := scanner.Err(); err != nil {
				return "", err
			}
		}
	}

	return "", errors.New("VKOSPI code not found in idxcode master")
}

func extractIndexCode(line string) string {
	matches := indexCodePattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	fullCode := matches[1]
	if len(fullCode) != 5 {
		return ""
	}
	return fullCode[1:]
}

func normalizeCandidates(candidates []string) []string {
	if len(candidates) == 0 {
		return defaultVKOSPICandidates
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(candidates))
	for _, code := range candidates {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}

	if len(out) == 0 {
		return defaultVKOSPICandidates
	}
	return out
}

func toRows(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
		return rows
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}

func extractCloseSeries(rows []map[string]any) ([]float64, error) {
	type point struct {
		Date  string
		Close float64
	}

	points := make([]point, 0, len(rows))
	for _, row := range rows {
		closeValue, ok := parseFloat(row["stck_clpr"])
		if !ok {
			continue
		}

		date := strings.TrimSpace(toString(row["stck_bsop_date"]))
		points = append(points, point{Date: date, Close: closeValue})
	}

	if len(points) == 0 {
		return nil, errors.New("no valid close prices found in output2")
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	closes := make([]float64, 0, len(points))
	for _, p := range points {
		closes = append(closes, p.Close)
	}
	return closes, nil
}

func calculateRSI(closes []float64, period int) (float64, error) {
	if len(closes) <= period {
		return 0, fmt.Errorf("not enough closes for RSI(%d): got %d", period, len(closes))
	}

	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		delta := closes[i] - closes[i-1]
		if delta > 0 {
			gainSum += delta
		} else if delta < 0 {
			lossSum += -delta
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	for i := period + 1; i < len(closes); i++ {
		delta := closes[i] - closes[i-1]

		gain := math.Max(delta, 0)
		loss := math.Max(-delta, 0)

		avgGain = ((avgGain * float64(period-1)) + gain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + loss) / float64(period)
	}

	if avgLoss == 0 {
		if avgGain == 0 {
			return 50, nil
		}
		return 100, nil
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	return rsi, nil
}

type RSIPoint struct {
	Date   string  `json:"date"`
	Close  float64 `json:"close"`
	RSI    float64 `json:"rsi"`
	Signal string  `json:"signal"`
}

// CalculateRSISeries는 기본 Wilder 평활법으로 역사적 RSI 시계열을 도출합니다.
func CalculateRSISeries(dates []string, closes []float64, period int) ([]RSIPoint, error) {
	return CalculateRSISeriesWithMethod(dates, closes, period, "wilder")
}

// CalculateRSISeriesWithMethod는 산출 방식("wilder" 또는 "sma"/"cutler")을 선택하여 역사적 RSI 시계열을 도출합니다.
func CalculateRSISeriesWithMethod(dates []string, closes []float64, period int, method string) ([]RSIPoint, error) {
	if period <= 0 {
		return nil, fmt.Errorf("period must be > 0")
	}
	if len(closes) <= period || len(closes) != len(dates) {
		return nil, fmt.Errorf("insufficient or mismatched data: closes=%d, dates=%d, period=%d", len(closes), len(dates), period)
	}

	calcRSIVal := func(g, l float64) float64 {
		if l == 0 {
			if g == 0 {
				return 50.0
			}
			return 100.0
		}
		rs := g / l
		return 100.0 - (100.0 / (1.0 + rs))
	}

	var series []RSIPoint
	method = strings.ToLower(strings.TrimSpace(method))

	if method == "sma" || method == "cutler" {
		for i := period; i < len(closes); i++ {
			var gSum, lSum float64
			for j := i - period + 1; j <= i; j++ {
				delta := closes[j] - closes[j-1]
				if delta > 0 {
					gSum += delta
				} else if delta < 0 {
					lSum += -delta
				}
			}
			avgG := gSum / float64(period)
			avgL := lSum / float64(period)
			rsiVal := calcRSIVal(avgG, avgL)

			series = append(series, RSIPoint{
				Date:   dates[i],
				Close:  closes[i],
				RSI:    math.Round(rsiVal*100) / 100,
				Signal: rsiSignal(rsiVal),
			})
		}
		return series, nil
	}

	// Default: Wilder's Exponential Smoothing
	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		delta := closes[i] - closes[i-1]
		if delta > 0 {
			gainSum += delta
		} else if delta < 0 {
			lossSum += -delta
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	initRSI := calcRSIVal(avgGain, avgLoss)
	series = append(series, RSIPoint{
		Date:   dates[period],
		Close:  closes[period],
		RSI:    math.Round(initRSI*100) / 100,
		Signal: rsiSignal(initRSI),
	})

	for i := period + 1; i < len(closes); i++ {
		delta := closes[i] - closes[i-1]
		gain := math.Max(delta, 0)
		loss := math.Max(-delta, 0)

		avgGain = ((avgGain * float64(period-1)) + gain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + loss) / float64(period)

		rsiVal := calcRSIVal(avgGain, avgLoss)
		series = append(series, RSIPoint{
			Date:   dates[i],
			Close:  closes[i],
			RSI:    math.Round(rsiVal*100) / 100,
			Signal: rsiSignal(rsiVal),
		})
	}

	return series, nil
}

func rsiSignal(rsi float64) string {
	switch {
	case rsi >= 70:
		return "OVERBOUGHT"
	case rsi <= 30:
		return "OVERSOLD"
	default:
		return "NEUTRAL"
	}
}

func firstOutputRow(resp *auth.RESTResponse, outputKey string) map[string]any {
	if resp == nil {
		return nil
	}

	rows := toRows(resp.Body[outputKey])
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func getEnvInt(key string, defaultValue int) int {
	return envcfg.Int(key, defaultValue)
}

func getOrDefaultEnv(key string, defaultValue string) string {
	return envcfg.Get(key, defaultValue)
}

func actualPBRDebugEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(kospiActualPBRDebugEnvKey)))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func actualPBRProgressEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(kospiActualPBRProgressEnvKey)))
	switch value {
	case "", "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return true
	}
}

func actualPBRProgressLog(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	log.Printf("[KOSPIActualPBR] "+format, args...)
}

func actualPBRLog(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	log.Printf("[KOSPIActualPBR] "+format, args...)
}

func parseFloat(value any) (float64, bool) {
	return parse.Float(value)
}

func toString(value any) string {
	return parse.String(value)
}

func isYYYYMMDD(value string) bool {
	if len(value) != 8 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
