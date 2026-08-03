package domesticfutureoption

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
)

func (s *Service) InquirePrice(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
	}
	return s.client.Get(ctx, inquirePricePath, inquirePriceTRID, "", params)
}

func (s *Service) InquireTimeFuopCCNL(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	params, err := futureCodeParams(marketDivCode, inputISCD)
	if err != nil {
		return nil, err
	}
	return s.client.Get(ctx, inquireTimeFuopCCNLPath, inquireTimeFuopCCNLTRID, "", params)
}

func (s *Service) InquireMember(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	params, err := futureCodeParams(marketDivCode, inputISCD)
	if err != nil {
		return nil, err
	}
	return s.client.Get(ctx, inquireMemberPath, inquireMemberTRID, "", params)
}

func (s *Service) InquireTimeFuopChartPrice(
	ctx context.Context,
	marketDivCode string,
	inputISCD string,
	hourClsCode string,
	includePastData string,
	includeFakeTick string,
	inputDate string,
	inputHour string,
) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	hourClsCode = strings.TrimSpace(hourClsCode)
	includePastData = strings.TrimSpace(includePastData)
	includeFakeTick = strings.TrimSpace(includeFakeTick)
	inputDate = strings.TrimSpace(inputDate)
	inputHour = strings.TrimSpace(inputHour)

	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}
	if hourClsCode == "" {
		hourClsCode = defaultTimeChartHourClassCode
	}
	if includePastData == "" {
		includePastData = defaultTimeChartIncludePastData
	}
	if includeFakeTick == "" {
		includeFakeTick = defaultTimeChartIncludeFakeTick
	}
	if inputDate == "" {
		inputDate = time.Now().Format("20060102")
	}
	if inputHour == "" {
		inputHour = time.Now().Format("150405")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
		"FID_HOUR_CLS_CODE":      hourClsCode,
		"FID_PW_DATA_INCU_YN":    includePastData,
		"FID_FAKE_TICK_INCU_YN":  includeFakeTick,
		"FID_INPUT_DATE_1":       inputDate,
		"FID_INPUT_HOUR_1":       inputHour,
	}
	return s.client.Get(ctx, inquireTimeFuopChartPricePath, inquireTimeFuopChartPriceTRID, "", params)
}

func (s *Service) DisplayBoardTop(
	ctx context.Context,
	marketDivCode string,
	inputISCD string,
	condMarketDivCode1 string,
	screenDivCode string,
	maturityCount string,
	condMarketClassCode string,
) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE":  marketDivCode,
		"FID_INPUT_ISCD":          inputISCD,
		"FID_COND_MRKT_DIV_CODE1": strings.TrimSpace(condMarketDivCode1),
		"FID_COND_SCR_DIV_CODE":   strings.TrimSpace(screenDivCode),
		"FID_MTRT_CNT":            strings.TrimSpace(maturityCount),
		"FID_COND_MRKT_CLS_CODE":  strings.TrimSpace(condMarketClassCode),
	}
	return s.client.Get(ctx, displayBoardTopPath, displayBoardTopTRID, "", params)
}

func (s *Service) DisplayBoardFutures(ctx context.Context, marketDivCode string, screenDivCode string, marketClassCode string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	screenDivCode = strings.TrimSpace(screenDivCode)
	marketClassCode = strings.TrimSpace(marketClassCode)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if screenDivCode == "" {
		screenDivCode = defaultBoardScreenCode
	}
	if marketClassCode == "" {
		marketClassCode = defaultBoardMarketClassCode
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_COND_SCR_DIV_CODE":  screenDivCode,
		"FID_COND_MRKT_CLS_CODE": marketClassCode,
	}
	return s.client.Get(ctx, displayBoardFuturesPath, displayBoardFuturesTRID, "", params)
}

func (s *Service) ExpPriceTrend(ctx context.Context, inputISCD string, marketDivCode string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_INPUT_ISCD":         inputISCD,
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
	}
	return s.client.Get(ctx, expPriceTrendPath, expPriceTrendTRID, "", params)
}

func futureCodeParams(marketDivCode string, inputISCD string) (map[string]string, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_INPUT_ISCD": inputISCD,
	}
	if marketDivCode != "" {
		params["FID_COND_MRKT_DIV_CODE"] = marketDivCode
	}
	return params, nil
}
