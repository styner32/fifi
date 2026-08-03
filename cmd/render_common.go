package main

import (
	"fmt"
	"strings"
)

func renderAuthTokenSummary(tokenCachePath string, accessToken string, validUntil string) {
	fmt.Printf("\n--- Auth Token Issued ---\n")
	fmt.Printf("Token Cache File: %s\n", tokenCachePath)
	fmt.Printf("Access Token (prefix): %.16s...\n", accessToken)
	fmt.Printf("Valid Until: %s\n", validUntil)
	fmt.Printf("-------------------------\n")
}

func printUsefulEndpoints() {
	fmt.Printf("\n--- Useful Endpoints For Market Status ---\n")
	endpoints := []struct {
		Path    string
		Purpose string
	}{
		{"/uapi/domestic-stock/v1/quotations/market-time", "장 운영 시간/영업일"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-price", "KOSPI/KOSDAQ/KOSPI200/VKOSPI 현재지수"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-timeprice", "업종 분봉 지수"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-daily-price", "업종 일/주/월 지수"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-tickprice", "업종 틱 지수"},
		{"/uapi/domestic-stock/v1/quotations/comp-program-trade-today", "프로그램매매 시간대 동향"},
		{"/uapi/domestic-stock/v1/quotations/comp-program-trade-daily", "프로그램매매 일별 동향"},
		{"/uapi/domestic-stock/v1/quotations/investor-program-trade-today", "투자자 프로그램매매 당일 동향"},
		{"/uapi/domestic-stock/v1/quotations/inquire-investor-daily-by-market", "시장별 투자자매매 동향"},
		{"/uapi/domestic-stock/v1/quotations/foreign-institution-total", "외인/기관 매매집계"},
		{"/uapi/domestic-stock/v1/quotations/inquire-investor", "종목/지수 투자자 수급"},
		{"/uapi/domestic-stock/v1/quotations/frgnmem-trade-estimate", "외국계 매매 가집계"},
		{"/uapi/domestic-stock/v1/quotations/investor-trend-estimate", "종목 외인/기관 추정 집계"},
		{"/uapi/domestic-stock/v1/quotations/inquire-vi-status", "VI 발동 현황"},
		{"/uapi/domestic-stock/v1/quotations/mktfunds", "증시자금 종합"},
		{"/uapi/domestic-stock/v1/quotations/inquire-asking-price-exp-ccn", "동시호가 예상체결/호가"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-price", "국내선물 현재가/베이시스"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopchartprice", "국내선물 분봉/체결 추세"},
		{"/uapi/domestic-futureoption/v1/quotations/display-board-top", "국내선물 기초자산/잔존일"},
		{"/uapi/domestic-futureoption/v1/quotations/display-board-futures", "국내선물 전광판"},
		{"/uapi/domestic-futureoption/v1/quotations/exp-price-trend", "국내선물 예상체결 추이"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopccnl", "국내선물 체결 추이(실험적 wrapper)"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-member", "국내선물 투자자별 매매동향(실험적 wrapper)"},
		{"/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice", "RSI/기술지표 원천(OHLCV)"},
		{"/uapi/domestic-stock/v1/finance/balance-sheet", "DCF용 자산/부채/자본 구조"},
		{"/uapi/domestic-stock/v1/finance/income-statement", "DCF용 매출/영업이익/감가상각"},
		{"/uapi/domestic-stock/v1/finance/other-major-ratios", "DCF용 EBITDA/EV-EBITDA proxy"},
		{"/uapi/domestic-stock/v1/finance/stability-ratio", "DCF용 차입금의존도 proxy"},
		{"/uapi/domestic-stock/v1/quotations/comp-interest", "DCF용 무위험금리(국내채권 금리)"},
		{"/uapi/domestic-bond/v1/quotations/inquire-price", "DCF용 무위험금리(국내채권 코드 직접 조회)"},
		{"/uapi/domestic-stock/v1/quotations/inquire-time-itemconclusion", "종목 체결 시계열(초단위)"},
		{"/uapi/domestic-stock/v1/quotations/pbar-tratio", "매물대/거래비중"},
		{"/uapi/domestic-stock/v1/quotations/tradprt-byamt", "체결금액대별 매매비중"},
		{"/uapi/domestic-stock/v1/ranking/volume-power", "체결강도 상위"},
		{"/uapi/domestic-stock/v1/ranking/volume-rank", "거래량 순위"},
		{"/uapi/domestic-stock/v1/ranking/fluctuation", "상승/하락률 랭킹"},
		{"/uapi/domestic-stock/v1/ranking/near-new-highlow", "신고/신저 근접"},
		{"/uapi/domestic-stock/v1/ranking/market-cap", "시가총액 상위"},
		{"/uapi/domestic-stock/v1/quotations/capture-uplowprice", "상/하한가 포착"},
	}

	for _, item := range endpoints {
		fmt.Printf("- %s : %s\n", item.Path, item.Purpose)
	}
	fmt.Printf("------------------------------------------\n")
}

func printSummaryBlock(title string, lines []string) {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 {
		return
	}

	fmt.Printf("\n--- %s ---\n", title)
	for _, line := range filtered {
		fmt.Println(line)
	}
	fmt.Printf("%s\n", strings.Repeat("-", len(title)+8))
}

func formatSummaryLine(label string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", label, value)
}
