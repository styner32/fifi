package snapshot

import (
	"fmt"
	"strings"
)

// Render는 스냅샷을 마크다운으로 렌더링합니다.
// prev가 제공되면 주요 지표에 전일 대비 변화를 함께 표시합니다.
func Render(s *Snapshot, prev ...*SnapshotJSON) string {
	if s == nil {
		_, ts := normalizeDate("")
		s = &Snapshot{Timestamp: ts, Errors: map[string]error{}}
	}
	var p *SnapshotJSON
	if len(prev) > 0 {
		p = prev[0]
	}
	var b strings.Builder
	header := "# Market Snapshot — " + s.Timestamp.Format(kstLayout)
	if p != nil {
		header += fmt.Sprintf("  (전일: %s)", p.Date)
	}
	b.WriteString(header + "\n\n")
	renderPrice(&b, s, p)
	renderFlow(&b, s, p)
	renderImpact(&b, s)
	renderGlobal(&b, s)
	renderCumulative(&b, s)
	renderMacro(&b, s)
	renderVolatility(&b, s, p)
	renderCredit(&b, s, p)
	renderRegime(&b, s, p)
	renderConcentration(&b, s, p)
	renderLateSession(&b, s, p)
	return b.String()
}

func renderPrice(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 1. 일중 가격 흐름\n")
	if s.Price == nil {
		b.WriteString("- " + na(sectionErr(s, "price")) + "\n\n")
		return
	}
	pr := s.Price
	highNote := ""
	if pr.YearHigh {
		highNote = " (연중 최고)"
	}
	b.WriteString(fmt.Sprintf("- 시가: %s (%s)\n", number(pr.Open, 2), percent((pr.Open/pr.PreviousClose-1)*100)))
	b.WriteString(fmt.Sprintf("- 고가: %s%s\n", number(pr.High, 2), highNote))
	b.WriteString(fmt.Sprintf("- 저가: %s\n", number(pr.Low, 2)))

	// 전일 종가: KIS API의 stck_prdy_clpr (pr.PreviousClose)가 권위값.
	// 저장된 JSON의 p.Price.Close는 스냅샷 실행 시점에 따라 stale할 수 있음.
	closeLine := number(pr.Close, 2)
	changePt := pr.Close - pr.PreviousClose
	changePct := changePt / pr.PreviousClose * 100
	closeLine += fmt.Sprintf("  [전일 종가 %s, %s (%s)]",
		number(pr.PreviousClose, 2), signedNumber(changePt, 2), percent(changePct))

	// 저장된 전일 스냅샷과 교차 검증 (off-by-one 감지)
	if p != nil && p.Price != nil && p.Price.Close > 0 {
		prevJSON := p.Price.Close
		if pr.PreviousClose > 0 {
			divergence := (prevJSON - pr.PreviousClose) / pr.PreviousClose * 100
			if divergence > 0.5 || divergence < -0.5 {
				closeLine += fmt.Sprintf("\n  - ⚠ 전일 스냅샷(%s) 종가 %s와 KIS 전일종가 %s 불일치 (%.2f%%) — 스냅샷 저장 시점 오류 가능",
					p.Date, number(prevJSON, 2), number(pr.PreviousClose, 2), divergence)
			}
		}
	}
	b.WriteString("- 종가: " + closeLine + "\n")
	b.WriteString(fmt.Sprintf("- 일중 변동폭: %sp (%s)\n\n", number(pr.RangePoints, 2), percentPlain(pr.RangePercent)))
}

func renderFlow(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 2. 수급 (코스피)\n| 주체 | 순매수 (억원) | 전일 |\n|---|---|---|\n")
	if s.Flow == nil {
		reason := na(sectionErr(s, "flow"))
		b.WriteString("| 외국인 | " + reason + " | - |\n| 기관 | " + reason + " | - |\n| 개인 | " + reason + " | - |\n\n")
		return
	}
	fmtFlowRow := func(name string, current float64, prevFunc func() float64) {
		prevStr := "-"
		if p != nil && p.Flow != nil {
			pv := prevFunc()
			prevStr = eok(pv)
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, eok(current), prevStr))
	}
	fmtFlowRow("외국인", s.Flow.ForeignEok, func() float64 { return p.Flow.ForeignEok })
	fmtFlowRow("기관", s.Flow.InstitutionEok, func() float64 { return p.Flow.InstitutionEok })
	fmtFlowRow("개인", s.Flow.IndividualEok, func() float64 { return p.Flow.IndividualEok })
	b.WriteString("\n")
}

func renderImpact(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 3. 시장 충격 지표\n")
	i := s.Impact
	if i == nil {
		b.WriteString("- " + na("impact section unavailable") + "\n\n")
		return
	}
	tvLine := na(i.ForeignSellTradingValueReason)
	if i.ForeignNetFlowToTradingValue != nil {
		tvLine = signedNumber(*i.ForeignNetFlowToTradingValue, 2) + "% [" + i.ForeignSellTradingValueLabel + "]"
	}
	b.WriteString("- 외국인 순수급/거래대금: " + tvLine + "\n")
	b.WriteString("  - 임계값: |순수급/거래대금| <10% 정상, 10~20% 주의, >20% 위험\n")
	
	mcVal := valuePercent(i.ForeignNetFlowToMarketCap, i.ForeignSellReason)
	if i.ForeignNetFlowToMarketCap != nil {
		mcVal = signedNumber(*i.ForeignNetFlowToMarketCap, 3) + "%"
	}
	b.WriteString("- 외국인 순수급/시가총액: " + mcVal + " (참고)\n")
	semiconductor := valuePercent(i.SemiconductorSellConcentrationPct, i.SemiconductorReason)
	if i.SemiconductorSellConcentrationPct != nil {
		semiconductor += " (외인 매도의)"
	}
	b.WriteString("- 반도체 매도 집중도: " + semiconductor + "\n")
	b.WriteString("- 코스피200 선물 변동률: " + quotePercent(i.FuturesChangePercent, i.FuturesReason) + "\n")
	// 베이시스: Section 11 (LateSession)에서 KOSPI200 코드 "2001"로 정확히 계산한 값 참조
	if s.LateSession != nil && s.LateSession.SpotPrice > 0 {
		basisText := fmt.Sprintf("%sp (%s) (판정 보류 — 이론 베이시스 미수집)", signedNumber(s.LateSession.BasisPoint, 1), percent(s.LateSession.BasisRate))
		b.WriteString(fmt.Sprintf("- 선물-현물 베이시스: %s — Section 11 참조\n", basisText))
	} else {
		b.WriteString("- 선물-현물 베이시스: Section 11 참조\n")
	}
	sidecar := na("manual input not provided")
	if i.SidecarStatus == "triggered" {
		sidecar = appendReason("✓", i.SidecarTime)
	} else if i.SidecarStatus == "not-triggered" {
		sidecar = "-"
	}
	b.WriteString("- 매도 사이드카 발동: " + sidecar + "\n\n")
}


func renderGlobal(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 4. 글로벌 동조성 (전일 대비)\n| 자산 | 변동률 | 비교 기준 | 기준 시각 |\n|---|---|---|---|\n")
	for _, item := range []struct{ label, symbol, basis, timeStr string }{
		{"닛케이225", "^N225", "전일 종가", "15:00 JST"},
		{"나스닥 선물", "NQ=F", "전일 선물 정산가", "실시간"},
		{"WTI 유가", "CL=F", "전일 정산가", "실시간"},
		{"BTC", "BTC-USD", "24시간 전", "실시간"},
		{"USD/KRW", "KRW=X", "Yahoo KRW=X 호가", "실시간"},
	} {
		value := na(sectionErr(s, "global"))
		if s.Global != nil {
			if q, ok := s.Global.Quotes[item.symbol]; ok {
				value = percent(q.ChangePercent)
				if item.symbol == "KRW=X" {
					value += " (" + number(q.Price, 2) + ")"
				}
			} else {
				value = na(s.Global.Reason)
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", item.label, value, item.basis, item.timeStr))
	}
	b.WriteString("\n")
}
