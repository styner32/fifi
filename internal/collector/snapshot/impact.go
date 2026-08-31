package snapshot

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type ImpactSection struct {
	// 거래대금 대비 외인 순수급 강도 (주요 지표)
	ForeignNetFlowToTradingValue  *float64 `json:"foreign_net_flow_to_trading_value"`
	ForeignSellTradingValueLabel  string   `json:"foreign_sell_trading_value_label"` // "BELOW_CUSTOM_ALERT_THRESHOLD"/"주의"/"위험"
	ForeignSellTradingValuePolicy string   `json:"foreign_sell_trading_value_policy"` // "abs(ratio)<10%"
	ForeignSellTradingValueReason string   `json:"foreign_sell_trading_value_reason,omitempty"`
	// 시총 대비 외인 순수급 (참고용)
	ForeignNetFlowToMarketCap *float64 `json:"foreign_net_flow_to_market_cap"`
	TotalKospiMarketCapEok    float64  `json:"total_kospi_market_cap_eok,omitempty"`
	ForeignSellReason         string   `json:"foreign_sell_reason,omitempty"`
	// 반도체
	SemiconductorSellConcentrationPct *float64 `json:"semiconductor_sell_concentration_pct"`
	SemiconductorReason               string   `json:"semiconductor_reason,omitempty"`
	// 선물
	FuturesChangePercent *float64 `json:"futures_change_percent"`
	FuturesPrice         *float64 `json:"futures_price"`
	FuturesCode          string   `json:"futures_code"`
	FuturesReason        string   `json:"futures_reason,omitempty"`
	// 사이드카
	SidecarStatus    string `json:"sidecar_status"`
	SidecarTime      string `json:"sidecar_time,omitempty"`
	EligibilityState string `json:"eligibility_state"` // "EXPIRED_FOR_DAY"
	TriggerState     string `json:"trigger_state"`     // "TRIGGER_HISTORY_UNKNOWN"
}

// collectImpact computes trading-value sell pressure and semiconductor concentration.
// price is used for KOSPI daily trading value (Fix 2).
func collectImpact(ctx context.Context, deps Deps, date string, flow *FlowSection, price *PriceSection, opts Options) *ImpactSection {
	section := &ImpactSection{
		SidecarStatus:    normalizeSidecar(opts.SidecarStatus),
		SidecarTime:      strings.TrimSpace(opts.SidecarTime),
		EligibilityState: "EXPIRED_FOR_DAY",
		TriggerState:     "TRIGGER_HISTORY_UNKNOWN",
	}
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
			section.ForeignSellTradingValuePolicy = "abs(ratio)<10%"
		} else {
			section.ForeignSellTradingValueReason = "trading value unavailable"
		}
		// 시총 대비 (참고)
		if deps.DomesticStock == nil {
			section.ForeignSellReason = "domestic stock dependency is nil"
		} else if summary, err := deps.DomesticStock.KOSPIMarketCapSummary(ctx, date); err != nil {
			section.ForeignSellReason = err.Error()
		} else {
			section.TotalKospiMarketCapEok = summary.TotalMarketCap
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
	return section
}

func tradingValueLabel(pct float64) string {
	absPct := math.Abs(pct)
	switch {
	case absPct < 10:
		if pct < 0 {
			return "BELOW_CUSTOM_ALERT_THRESHOLD (순매도)"
		}
		return "BELOW_CUSTOM_ALERT_THRESHOLD [정상]"
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
