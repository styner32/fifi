package domesticstock

import (
	"context"
	"errors"
	"strings"

	"github.com/fifi/internal/auth"
)

const (
	inquireInvestorPath         = "/uapi/domestic-stock/v1/quotations/inquire-investor"
	investorDailyByMarketPath   = "/uapi/domestic-stock/v1/quotations/inquire-investor-daily-by-market"
	foreignInstitutionTotalPath = "/uapi/domestic-stock/v1/quotations/foreign-institution-total"
	askingPriceExpCCNPath       = "/uapi/domestic-stock/v1/quotations/inquire-asking-price-exp-ccn"
	inquireInvestorTRID         = "FHKST01010900"
	investorDailyByMarketTRID   = "FHPTJ04040000"
	foreignInstitutionTotalTRID = "FHPTJ04400000"
	askingPriceExpCCNTRID       = "FHKST01010200"
	defaultForeignMarketDivCode = "V"
	defaultForeignScreenDivCode = "16449"
	defaultForeignInputISCD     = "0000"
	defaultForeignDivClassCode  = "1"
	defaultForeignRankSortCode  = "0"
	defaultForeignEtcClassCode  = "1"
)

func (s *Service) InquireInvestor(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = "J"
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
	}
	return s.client.Get(ctx, inquireInvestorPath, inquireInvestorTRID, "", params)
}

func (s *Service) InquireInvestorDailyByMarket(ctx context.Context, yyyymmdd string) (*auth.RESTResponse, error) {
	yyyymmdd = strings.TrimSpace(yyyymmdd)
	if yyyymmdd == "" {
		return nil, errors.New("yyyymmdd is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "U",
		"FID_INPUT_ISCD":         "0001",
		"FID_INPUT_DATE_1":       yyyymmdd,
		"FID_INPUT_ISCD_1":       "KSP",
		"FID_INPUT_DATE_2":       yyyymmdd,
		"FID_INPUT_ISCD_2":       "0001",
	}
	return s.client.Get(ctx, investorDailyByMarketPath, investorDailyByMarketTRID, "", params)
}

func (s *Service) ForeignInstitutionTotal(
	ctx context.Context,
	marketDivCode string,
	screenDivCode string,
	inputISCD string,
	divClassCode string,
	rankSortCode string,
	etcClassCode string,
) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	screenDivCode = strings.TrimSpace(screenDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	divClassCode = strings.TrimSpace(divClassCode)
	rankSortCode = strings.TrimSpace(rankSortCode)
	etcClassCode = strings.TrimSpace(etcClassCode)

	if marketDivCode == "" {
		marketDivCode = defaultForeignMarketDivCode
	}
	if screenDivCode == "" {
		screenDivCode = defaultForeignScreenDivCode
	}
	if inputISCD == "" {
		inputISCD = defaultForeignInputISCD
	}
	if divClassCode == "" {
		divClassCode = defaultForeignDivClassCode
	}
	if rankSortCode == "" {
		rankSortCode = defaultForeignRankSortCode
	}
	if etcClassCode == "" {
		etcClassCode = defaultForeignEtcClassCode
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_COND_SCR_DIV_CODE":  screenDivCode,
		"FID_INPUT_ISCD":         inputISCD,
		"FID_DIV_CLS_CODE":       divClassCode,
		"FID_RANK_SORT_CLS_CODE": rankSortCode,
		"FID_ETC_CLS_CODE":       etcClassCode,
	}
	return s.client.Get(ctx, foreignInstitutionTotalPath, foreignInstitutionTotalTRID, "", params)
}

func (s *Service) InquireAskingPriceExpCCN(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = "J"
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
	}
	return s.client.Get(ctx, askingPriceExpCCNPath, askingPriceExpCCNTRID, "", params)
}

func (s *Service) InvestorProgramTradeToday(ctx context.Context, marketDiv string) (*auth.RESTResponse, error) {
	marketDiv = strings.TrimSpace(marketDiv)
	if marketDiv == "" {
		return nil, errors.New("marketDiv is required (1: KOSPI, 4: KOSDAQ)")
	}
	params := map[string]string{
		"MRKT_DIV_CLS_CODE": marketDiv,
		"EXCH_DIV_CLS_CODE": "J",
	}
	return s.client.Get(ctx, investorProgramTradeTodayPath, investorProgramTradeTodayTRID, "", params)
}

func (s *Service) InquireInvestorTimeByMarket(ctx context.Context, marketDiv string, indexDiv string) (*auth.RESTResponse, error) {
	marketDiv = strings.TrimSpace(marketDiv)
	indexDiv = strings.TrimSpace(indexDiv)
	if marketDiv == "" || indexDiv == "" {
		return nil, errors.New("marketDiv and indexDiv are required")
	}
	params := map[string]string{
		"FID_INPUT_ISCD":   marketDiv,
		"FID_INPUT_ISCD_2": indexDiv,
	}
	return s.client.Get(ctx, investorTimeByMarketPath, investorTimeByMarketTRID, "", params)
}
