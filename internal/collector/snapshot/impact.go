package snapshot

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type ImpactSection struct {
	// 거래대금 대비 외인 순수급 강도 (주요 지표)
	ForeignNetFlowToTradingValue *float64
	ForeignSellTradingValueLabel string // "정상"/"주의"/"위험"
	ForeignSellTradingValueReason string
	// 시총 대비 외인 순수급 (참고용)
	ForeignNetFlowToMarketCap *float64
	ForeignSellReason         string
	// 반도체
	SemiconductorSellConcentrationPct *float64
	SemiconductorReason               string
	// 선물
	FuturesChangePercent *float64
	FuturesPrice         *float64
	FuturesCode          string
	FuturesReason        string
	// 사이드카
	SidecarStatus string
	SidecarTime   string
	// 베이시스는 Section 11 (LateSession)으로 통합 — 중복 API 호출 및 코드 불일치(0002 vs 2001) 제거
}

// collectImpact computes trading-value sell pressure and semiconductor concentration.
// price is used for KOSPI daily trading value (Fix 2).
func collectImpact(ctx context.Context, deps Deps, date string, flow *FlowSection, price *PriceSection, opts Options) *ImpactSection {
	section := &ImpactSection{SidecarStatus: normalizeSidecar(opts.SidecarStatus), SidecarTime: strings.TrimSpace(opts.SidecarTime)}
	if flow == nil {
		section.ForeignSellReason = "flow section unavailable"
		section.ForeignSellTradingValueReason = "flow section unavailable"
		section.SemiconductorReason = "flow section unavailable"
	} else {
		// Fix 2: 거래대금 분모
		if price != nil && price.TradingValueEok > 0 {
			pct := flow.ForeignEok / price.TradingValueEok * 100
			section.ForeignNetFlowToTradingValue = ptr(pct)
			section.ForeignSellTradingValueLabel = tradingValueLabel(pct)
		} else {
			section.ForeignSellTradingValueReason = "trading value unavailable"
		}
		// 시총 대비 (참고)
		if deps.DomesticStock == nil {
			section.ForeignSellReason = "domestic stock dependency is nil"
		} else if summary, err := deps.DomesticStock.KOSPIMarketCapSummary(ctx, date); err != nil {
			section.ForeignSellReason = err.Error()
		} else {
			section.ForeignNetFlowToMarketCap = signedPercent(flow.ForeignEok, summary.TotalMarketCap)
		}
		// 반도체
		if opts.SemiconductorForeignNetSellEok != nil {
			section.SemiconductorSellConcentrationPct = absPercent(*opts.SemiconductorForeignNetSellEok, flow.ForeignEok)
		} else {
			section.SemiconductorReason = "manual input not provided"
		}
	}
	section.collectFutures(ctx, deps.DomesticFuture, date)
	// 베이시스는 Section 11 (LateSession.fillBasis)에서 KOSPI200 코드 "2001"로 정확히 계산.
	// Section 3의 "0002" 코드 사용은 필드 불일치(현물=10057 vs 정상=1459)를 유발하여 제거됨.
	return section
}

func tradingValueLabel(pct float64) string {
	absPct := math.Abs(pct)
	switch {
	case absPct < 10:
		if pct < 0 {
			return "자체 경보 임계치 미만이나 의미 있는 순매도"
		}
		return "정상"
	case absPct < 20:
		return "주의"
	default:
		return "위험"
	}
}



func (s *ImpactSection) collectFutures(ctx context.Context, futures DomesticFuture, date string) {
	if futures == nil {
		s.FuturesReason = "domestic future dependency is nil"
		return
	}
	resolved, err := futures.ResolveNearMonthKOSPI200Futures(ctx, date)
	if err != nil {
		s.FuturesReason = err.Error()
		return
	}
	s.FuturesCode = resolved.Record.ShortCode
	resp, err := futures.InquirePrice(ctx, "F", resolved.Record.ShortCode)
	if err != nil {
		s.FuturesReason = err.Error()
		return
	}
	row := firstRow(resp, "output1", "output")
	if row == nil {
		s.FuturesReason = "futures price output missing"
		return
	}
	if value, ok := num(row, "futs_prdy_ctrt", "bstp_nmix_prdy_ctrt"); ok {
		s.FuturesChangePercent = ptr(value)
	} else {
		s.FuturesReason = "futs_prdy_ctrt missing"
	}
	// Fix 3: 선물 실제가 추출
	if price, ok := num(row, "futs_prpr", "bstp_nmix_prpr"); ok {
		s.FuturesPrice = ptr(price)
	}
}

// collectBasis: 제거됨 — Section 11 (LateSession.fillBasis)로 통합.
// Section 3의 InquireIndexPrice("0002")는 KOSPI200이 아닌 다른 지수를 반환하여
// "out of range -85.3%" 오류를 유발했음. Section 11의 "2001" 코드가 올바른 KOSPI200 조회.

func normalizeSidecar(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "triggered", "not-triggered":
		return strings.ToLower(strings.TrimSpace(value))
	case "", "unknown":
		return "unknown"
	default:
		return fmt.Sprintf("unknown (%s)", strings.TrimSpace(value))
	}
}
