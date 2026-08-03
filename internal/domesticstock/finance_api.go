package domesticstock

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fifi/internal/auth"
)

const (
	financeBalanceSheetPath    = "/uapi/domestic-stock/v1/finance/balance-sheet"
	financeIncomeStatementPath = "/uapi/domestic-stock/v1/finance/income-statement"
	defaultFinanceDivClsCode   = "0"
	defaultFinanceMarketDiv    = "J"
)

const (
	financeBalanceSheetTRID    = "FHKST66430100"
	financeIncomeStatementTRID = "FHKST66430200"
)

const (
	financeOtherMajorRatiosPath = "/uapi/domestic-stock/v1/finance/other-major-ratios"
	financeStabilityRatioPath   = "/uapi/domestic-stock/v1/finance/stability-ratio"
	compInterestPath            = "/uapi/domestic-stock/v1/quotations/comp-interest"
	indexDailyPricePath         = "/uapi/domestic-stock/v1/quotations/inquire-index-daily-price"

	financeOtherMajorRatiosTRID = "FHKST66430500"
	financeStabilityRatioTRID   = "FHKST66430600"
	compInterestTRID            = "FHPST07020000"
	indexDailyPriceTRID         = "FHPUP02120000"

	dcfRiskFreeRateEnvKey     = "DCF_RISK_FREE_RATE"
	dcfRiskFreeBondCodeEnvKey = "DCF_RISK_FREE_BOND_CODE"
	dcfRiskFreeBondDivEnvKey  = "DCF_RISK_FREE_BOND_MARKET_DIV_CODE"
	dcfRiskFreeNameHintEnvKey = "DCF_RISK_FREE_NAME_HINT"
	dcfBetaEnvKey             = "DCF_BETA"
	dcfMarketPremiumEnvKey    = "DCF_MARKET_PREMIUM"
	dcfCostOfDebtEnvKey       = "DCF_COST_OF_DEBT"
	dcfNetDebtEnvKey          = "DCF_NET_DEBT"
	dcfForecastYearsEnvKey    = "DCF_FORECAST_YEARS"
	dcfTerminalGrowthEnvKey   = "DCF_TERMINAL_GROWTH"
	dcfIndexCodeEnvKey        = "DCF_INDEX_CODE"
	dcfBetaLookbackEnvKey     = "DCF_BETA_LOOKBACK_DAYS"
	dcfCreditSpreadEnvKey     = "DCF_CREDIT_SPREAD"

	defaultDCFMarketPremium   = 0.055
	defaultDCFTerminalGrowth  = 0.02
	defaultDCFForecastYears   = 5
	defaultDCFBetaLookback    = 400
	defaultDCFCreditSpread    = 0.015
	defaultDCFIndexCode       = "0001"
	defaultDCFRiskFreeBondDiv = "B"
)

func (s *Service) FinanceBalanceSheet(ctx context.Context, symbol string, divClsCode string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	divClsCode = strings.TrimSpace(divClsCode)
	if divClsCode == "" {
		divClsCode = defaultFinanceDivClsCode
	}

	params := map[string]string{
		"FID_DIV_CLS_CODE":       divClsCode,
		"fid_cond_mrkt_div_code": defaultFinanceMarketDiv,
		"fid_input_iscd":         symbol,
	}
	return s.client.Get(ctx, financeBalanceSheetPath, financeBalanceSheetTRID, "", params)
}

func (s *Service) FinanceIncomeStatement(ctx context.Context, symbol string, divClsCode string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	divClsCode = strings.TrimSpace(divClsCode)
	if divClsCode == "" {
		divClsCode = defaultFinanceDivClsCode
	}

	params := map[string]string{
		"FID_DIV_CLS_CODE":       divClsCode,
		"fid_cond_mrkt_div_code": defaultFinanceMarketDiv,
		"fid_input_iscd":         symbol,
	}
	return s.client.Get(ctx, financeIncomeStatementPath, financeIncomeStatementTRID, "", params)
}

func (s *Service) FinanceOtherMajorRatios(ctx context.Context, symbol string, divClsCode string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	divClsCode = strings.TrimSpace(divClsCode)
	if divClsCode == "" {
		divClsCode = defaultFinanceDivClsCode
	}

	params := map[string]string{
		"fid_input_iscd":         symbol,
		"fid_div_cls_code":       divClsCode,
		"fid_cond_mrkt_div_code": defaultFinanceMarketDiv,
	}
	return s.client.Get(ctx, financeOtherMajorRatiosPath, financeOtherMajorRatiosTRID, "", params)
}

func (s *Service) FinanceStabilityRatio(ctx context.Context, symbol string, divClsCode string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	divClsCode = strings.TrimSpace(divClsCode)
	if divClsCode == "" {
		divClsCode = defaultFinanceDivClsCode
	}

	params := map[string]string{
		"fid_input_iscd":         symbol,
		"fid_div_cls_code":       divClsCode,
		"fid_cond_mrkt_div_code": defaultFinanceMarketDiv,
	}
	return s.client.Get(ctx, financeStabilityRatioPath, financeStabilityRatioTRID, "", params)
}

func (s *Service) CompInterest(ctx context.Context) (*auth.RESTResponse, error) {
	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "I",
		"FID_COND_SCR_DIV_CODE":  "20702",
		"FID_DIV_CLS_CODE":       "1",
		"FID_DIV_CLS_CODE1":      "",
	}
	return s.client.Get(ctx, compInterestPath, compInterestTRID, "", params)
}

func (s *Service) InquireIndexDailyPrice(ctx context.Context, indexCode string, fromDate string) ([]map[string]any, error) {
	return s.inquireIndexDailyPriceWithMarket(ctx, indexCode, fromDate, "U")
}

// InquireVKOSPIDailyPrice는 VKOSPI 일별 이력 조회입니다.
// idxcode.mst 기준 idx_div="0", idx_code="0503"이나 TRID는 "U" 마켓코드만 허용.
func (s *Service) InquireVKOSPIDailyPrice(ctx context.Context, indexCode string, fromDate string) ([]map[string]any, error) {
	return s.inquireIndexDailyPriceWithMarket(ctx, indexCode, fromDate, "U")
}

func (s *Service) inquireIndexDailyPriceWithMarket(ctx context.Context, indexCode string, fromDate string, marketDivCode string) ([]map[string]any, error) {
	indexCode = strings.TrimSpace(indexCode)
	fromDate = strings.TrimSpace(fromDate)
	if indexCode == "" {
		return nil, fmt.Errorf("indexCode is required")
	}
	if fromDate == "" {
		return nil, fmt.Errorf("fromDate is required")
	}

	params := map[string]string{
		"FID_PERIOD_DIV_CODE":    "D",
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         indexCode,
		"FID_INPUT_DATE_1":       fromDate,
	}

	rows := make([]map[string]any, 0, 256)
	trCont := ""
	for depth := 0; depth < 10; depth++ {
		resp, err := s.client.Get(ctx, indexDailyPricePath, indexDailyPriceTRID, trCont, params)
		if err != nil {
			return nil, err
		}
		rows = append(rows, toRows(resp.Body["output2"])...)
		headerValue := strings.TrimSpace(resp.Headers.Get("tr_cont"))
		if headerValue != "M" && headerValue != "F" {
			break
		}
		trCont = "N"
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("index daily price output2 missing for %s", indexCode)
	}
	return rows, nil
}

