package domesticstock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fifi/internal/auth"
)

func (s *Service) MarketTime(ctx context.Context) (*auth.RESTResponse, error) {
	return s.client.Get(ctx, marketTimePath, marketTimeTRID, "", map[string]string{})
}

func (s *Service) InquirePrice(ctx context.Context, symbol string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "J",
		"FID_INPUT_ISCD":         symbol,
	}
	return s.client.Get(ctx, inquirePricePath, inquirePriceTRID, "", params)
}

func (s *Service) InquireIndexPrice(ctx context.Context, indexCode string) (*auth.RESTResponse, error) {
	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "U",
		"FID_INPUT_ISCD":         strings.TrimSpace(indexCode),
	}
	return s.client.Get(ctx, inquireIndexPricePath, inquireIndexPriceTRID, "", params)
}

// InquireVKOSPIPrice는 VKOSPI 전용 조회입니다.
// idxcode.mst 구조: idx_div="0", idx_code="0503"
// 하지만 FHPUP02100000 TRID는 FID_COND_MRKT_DIV_CODE="U"만 허용.
// 실제 VKOSPI API 코드는 VKOSPI_INDEX_CODE 환경변수로 직접 지정 가능.
func (s *Service) InquireVKOSPIPrice(ctx context.Context, indexCode string) (*auth.RESTResponse, error) {
	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "U",
		"FID_INPUT_ISCD":         strings.TrimSpace(indexCode),
	}
	return s.client.Get(ctx, inquireIndexPricePath, inquireIndexPriceTRID, "", params)
}

func (s *Service) CompProgramTradeToday(ctx context.Context, marketClassCode string) (*auth.RESTResponse, error) {
	marketClassCode = strings.TrimSpace(marketClassCode)
	if marketClassCode == "" {
		marketClassCode = defaultProgramMarketClass
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE":  "J",
		"FID_MRKT_CLS_CODE":       marketClassCode,
		"FID_SCTN_CLS_CODE":       "",
		"FID_INPUT_ISCD":          "",
		"FID_COND_MRKT_DIV_CODE1": "",
		"FID_INPUT_HOUR_1":        "",
	}
	return s.client.Get(ctx, programTradeTodayPath, programTradeTodayTRID, "", params)
}

func (s *Service) InquireVIStatus(ctx context.Context, yyyymmdd string) (*auth.RESTResponse, error) {
	if strings.TrimSpace(yyyymmdd) == "" {
		return nil, errors.New("yyyymmdd is required")
	}

	params := map[string]string{
		"FID_DIV_CLS_CODE":       "0",
		"FID_COND_SCR_DIV_CODE":  "20139",
		"FID_MRKT_CLS_CODE":      "0",
		"FID_INPUT_ISCD":         "",
		"FID_RANK_SORT_CLS_CODE": "0",
		"FID_INPUT_DATE_1":       yyyymmdd,
		"FID_TRGT_CLS_CODE":      "",
		"FID_TRGT_EXLS_CLS_CODE": "",
	}
	return s.client.Get(ctx, viStatusPath, viStatusTRID, "", params)
}

func (s *Service) MarketFunds(ctx context.Context, yyyymmdd string) (*auth.RESTResponse, error) {
	params := map[string]string{
		"FID_INPUT_DATE_1": strings.TrimSpace(yyyymmdd),
	}
	return s.client.Get(ctx, marketFundsPath, marketFundsTRID, "", params)
}

// InquireCreditBalanceRanking fetches credit balance ranking (top stocks).
// TR: FHKST17010000 (국내주식 신용잔고 상위 [국내주식-109])
// marketCode: "0"=전체, "1"=KOSPI, "2"=KOSDAQ
func (s *Service) InquireCreditBalanceRanking(ctx context.Context, marketCode string) (*auth.RESTResponse, error) {
	// FID_INPUT_ISCD: "0000"=전체, "0001"=거래소, "1001"=코스닥, "2001"=코스피200
	inputISCD := "0000"
	if marketCode == "1" {
		inputISCD = "0001"
	} else if marketCode == "2" {
		inputISCD = "1001"
	}
	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "J",
		"FID_COND_SCR_DIV_CODE":  "11701",
		"FID_INPUT_ISCD":         inputISCD,
		"FID_RANK_SORT_CLS_CODE": "0",
		"FID_OPTION":             "2",
	}
	return s.client.Get(ctx, creditBalancePath, creditBalanceTRID, "", params)
}

// InquireDailyCreditBalance fetches per-stock daily credit balance history.
// TR: FHKST130400C0 (국내주식 신용잔고 일별추이 [국내주식-110])
func (s *Service) InquireDailyCreditBalance(ctx context.Context, symbol string, date string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if date == "" {
		date = "99991231"
	}
	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "J",
		"FID_COND_SCR_DIV_CODE":  "13040",
		"FID_INPUT_ISCD":         symbol,
		"FID_INPUT_DATE_1":       date,
	}
	return s.client.Get(ctx, dailyCreditBalancePath, dailyCreditBalanceTRID, "", params)
}
func (s *Service) InquireDailyItemChartPrice(
	ctx context.Context,
	symbol string,
	fromDate string,
	toDate string,
	period string,
	orgAdjPrice string,
) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	period = strings.TrimSpace(period)
	orgAdjPrice = strings.TrimSpace(orgAdjPrice)

	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if fromDate == "" || toDate == "" {
		return nil, errors.New("fromDate and toDate are required")
	}
	if period == "" {
		period = "D"
	}
	if orgAdjPrice == "" {
		orgAdjPrice = "1"
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "J",
		"FID_INPUT_ISCD":         symbol,
		"FID_INPUT_DATE_1":       fromDate,
		"FID_INPUT_DATE_2":       toDate,
		"FID_PERIOD_DIV_CODE":    period,
		"FID_ORG_ADJ_PRC":        orgAdjPrice,
	}
	return s.client.Get(ctx, dailyItemChartPricePath, dailyItemChartPriceTRID, "", params)
}

func (s *Service) ResolveVKOSPICode(ctx context.Context, candidates []string) (string, error) {
	s.vkospiMu.RLock()
	cached := s.resolvedVKOSPICode
	s.vkospiMu.RUnlock()
	if cached != "" {
		return cached, nil
	}

	var orderedCandidates []string
	seen := make(map[string]struct{})
	addCandidate := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		if _, exists := seen[c]; !exists {
			seen[c] = struct{}{}
			orderedCandidates = append(orderedCandidates, c)
		}
	}

	if envCode := os.Getenv(vkospiCodeEnvKey); envCode != "" {
		addCandidate(envCode)
	}
	for _, c := range candidates {
		addCandidate(c)
	}
	for _, c := range defaultVKOSPICandidates {
		addCandidate(c)
	}

	var probeErrors []string
	for _, code := range orderedCandidates {
		resp, reqErr := s.InquireVKOSPIPrice(ctx, code)
		if reqErr != nil {
			probeErrors = append(probeErrors, fmt.Sprintf("%s: request error: %v", code, reqErr))
			continue
		}
		if !resp.IsOK() {
			probeErrors = append(probeErrors, fmt.Sprintf("%s: api error msg_cd=%s msg1=%s", code, resp.MessageCode(), resp.Message()))
			continue
		}
		row := resp.FirstRow("output")
		if row == nil {
			probeErrors = append(probeErrors, fmt.Sprintf("%s: empty output row", code))
			continue
		}
		val, ok := parseFloat(row["bstp_nmix_prpr"])
		if !ok || val < 5 || val > 100 {
			probeErrors = append(probeErrors, fmt.Sprintf("%s: value %.2f (ok=%v) out of range (5~100)", code, val, ok))
			continue
		}

		s.vkospiMu.Lock()
		s.resolvedVKOSPICode = code
		s.vkospiMu.Unlock()
		return code, nil
	}

	discoveredCode, err := discoverVKOSPICodeFromMaster(ctx, s.client)
	if err == nil && discoveredCode != "" {
		if _, alreadyTested := seen[discoveredCode]; !alreadyTested {
			resp, reqErr := s.InquireVKOSPIPrice(ctx, discoveredCode)
			if reqErr == nil && resp.IsOK() {
				if row := resp.FirstRow("output"); row != nil {
					if val, ok := parseFloat(row["bstp_nmix_prpr"]); ok && val >= 5 && val <= 100 {
						s.vkospiMu.Lock()
						s.resolvedVKOSPICode = discoveredCode
						s.vkospiMu.Unlock()
						return discoveredCode, nil
					}
				}
			}
			probeErrors = append(probeErrors, fmt.Sprintf("master discovery %s failed validation", discoveredCode))
		}
	} else if err != nil {
		probeErrors = append(probeErrors, fmt.Sprintf("master discovery error: %v", err))
	}

	return "", fmt.Errorf("unable to resolve VKOSPI code (tested candidates: %s)", strings.Join(probeErrors, "; "))
}

func (s *Service) RSIFromDailyChart(
	ctx context.Context,
	symbol string,
	period int,
	fromDate string,
	toDate string,
) (*RSIResult, error) {
	if period <= 0 {
		return nil, errors.New("period must be > 0")
	}

	resp, err := s.InquireDailyItemChartPrice(ctx, symbol, fromDate, toDate, "D", "1")
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("RSI source API returned error: msg_cd=%s msg1=%s", resp.MessageCode(), resp.Message())
	}

	rows := toRows(resp.Body["output2"])
	if len(rows) == 0 {
		return nil, errors.New("daily chart output2 is empty")
	}

	closes, err := extractCloseSeries(rows)
	if err != nil {
		return nil, err
	}

	rsiValue, err := calculateRSI(closes, period)
	if err != nil {
		return nil, err
	}

	return &RSIResult{
		Symbol:     strings.TrimSpace(symbol),
		Period:     period,
		Last:       rsiValue,
		Signal:     rsiSignal(rsiValue),
		SampleSize: len(closes),
	}, nil
}
