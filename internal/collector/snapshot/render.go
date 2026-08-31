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
	timeStatus := " [MIXED_TIME_POST_CLOSE]"
	header := "# Market Snapshot — " + s.Timestamp.Format("2006-01-02 15:30 KST") + timeStatus
	if p != nil {
		header += fmt.Sprintf("  (전일: %s)", p.Date)
	}
	b.WriteString(header + "\n")
	b.WriteString("- 현물 기준 시각: " + s.Timestamp.Format("2006-01-02 15:30 KST") + "\n")
	b.WriteString("- 선물 최종 종가 시각: " + s.Timestamp.Format("2006-01-02 15:45 KST") + "\n\n")

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
	rangePct := pr.RangePercent
	if pr.IntradayRangePctOfPrevClose > 0 {
		rangePct = pr.IntradayRangePctOfPrevClose
	}
	b.WriteString(fmt.Sprintf("- 일중 변동폭 (intraday_range_points): %sp (전일 종가 대비 %.2f%%)\n\n", number(pr.RangePoints, 2), rangePct))
}

func renderFlow(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 2. 수급 (코스피)\n| 주체 | 순매수 (억원) | 전일 순매수 (억원) |\n|---|---:|---:|\n")
	if s.Flow == nil {
		reason := na(sectionErr(s, "flow"))
		b.WriteString("| 외국인 | " + reason + " | N/A |\n| 기관 | " + reason + " | N/A |\n| 개인 | " + reason + " | N/A |\n\n")
		return
	}
	hasPrev := p != nil && p.Flow != nil
	fmtFlowRow := func(name string, current float64, getPrev func() *float64) {
		prevStr := "N/A"
		if hasPrev {
			if pv := getPrev(); pv != nil {
				prevStr = eok(*pv)
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, eok(current), prevStr))
	}
	fmtFlowRow("외국인", s.Flow.ForeignEok, func() *float64 { return p.Flow.ForeignPrevEok })
	fmtFlowRow("기관", s.Flow.InstitutionEok, func() *float64 { return p.Flow.InstitutionPrevEok })
	fmtFlowRow("개인", s.Flow.IndividualEok, func() *float64 { return p.Flow.IndividualPrevEok })
	b.WriteString("\n")

	if s.Flow.DisplayedParticipantResidualEok != 0 || s.Flow.ResidualEok != 0 {
		resVal := s.Flow.DisplayedParticipantResidualEok
		if resVal == 0 {
			resVal = s.Flow.ResidualEok
		}
		b.WriteString(fmt.Sprintf("- 표시된 당일 3주체 합계 잔차: %s억원\n", eok(resVal)))
		b.WriteString(fmt.Sprintf("- 수급 정합성 상태: `%s`\n", s.Flow.ReconciliationStatus))
		b.WriteString("- 사유: 기타법인·기타외국인 등 미표시 참여자 또는 집계 범위 차이 가능\n")
	}
	if !hasPrev {
		b.WriteString("- 전일 수급 상태: `MISSING`\n")
		b.WriteString("- 사유: `PREVIOUS_DAY_FLOW_NOT_COLLECTED`\n")
	}
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
		label := i.ForeignSellTradingValueLabel
		if label == "" {
			label = tradingValueLabel(*i.ForeignNetFlowToTradingValue)
		}
		tvLine = signedNumber(*i.ForeignNetFlowToTradingValue, 2) + "% [" + label + "]"
	}
	b.WriteString("- 외국인 순수급/거래대금: " + tvLine + "\n")
	b.WriteString("  - 사용자 정의 임계값: 절댓값 10% 미만 정상, 10~20% 주의, 20% 초과 위험\n")

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
	b.WriteString("- 선물-현물 15:30 표기값 원시 스프레드: Section 11 참조 (RAW_SPREAD / ALIGNMENT_UNVERIFIED)\n")
	b.WriteString("- 매도 사이드카:\n")
	b.WriteString("  - 발동 가능 상태: `EXPIRED_FOR_DAY`\n")
	b.WriteString("  - 당일 발동 이력: `TRIGGER_HISTORY_UNKNOWN`\n")
	b.WriteString("  - 사유: `OFFICIAL_EVENT_HISTORY_NOT_COLLECTED`\n\n")
}

func renderGlobal(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 4. 글로벌 동조성 (전일 대비)\n| 자산 | 변동률 | 비교 기준 | 관측 시각 |\n|---|---|---|---|\n")
	for _, item := range []struct{ label, symbol, basis, defaultTime string }{
		{"닛케이225", "^N225", "전일 종가", "15:00 JST"},
		{"나스닥 선물", "NQ=F", "전일 선물 정산가", "TIMESTAMP_MISSING"},
		{"WTI 유가", "CL=F", "전일 정산가", "TIMESTAMP_MISSING"},
		{"BTC", "BTC-USD", "24시간 전", "TIMESTAMP_MISSING"},
		{"USD/KRW", "KRW=X", "Yahoo KRW=X 호가", "TIMESTAMP_MISSING"},
	} {
		value := na(sectionErr(s, "global"))
		timeStr := item.defaultTime
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
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", item.label, value, item.basis, timeStr))
	}
	b.WriteString("\n")
}
