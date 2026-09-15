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
	header := "# Market Snapshot — " + s.BusinessDate + " [" + s.SessionStatus + "]"
	if p != nil {
		header += fmt.Sprintf(" (이전 저장본: %s)", p.Date)
	}
	b.WriteString(header + "\n")
	b.WriteString("- 수집 시작: " + observationLabel(s.Timestamp) + "\n")
	b.WriteString("- 수집 완료: " + observationLabel(s.CompletedAt) + "\n")
	b.WriteString("- 각 항목은 출처의 기준일·관측 시각을 따릅니다. 조회 시각은 가격 확정 시각이 아닙니다.\n\n")

	if p != nil && p.SchemaVersion < 2 {
		b.WriteString("- 이전 저장본은 단위·산출 범위 검증 전 형식이므로 수치 비교에서 제외했습니다.\n\n")
		p = nil
	}
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
	if p != nil && p.SchemaVersion >= 2 && p.SessionStatus == "POST_CLOSE_UNRECONCILED" && p.Price != nil && p.Price.Date == p.Date && s.Flow != nil && s.Flow.PreviousDate == p.Date && p.Price.Close > 0 {
		prevJSON := p.Price.Close
		if pr.PreviousClose > 0 {
			divergence := (prevJSON - pr.PreviousClose) / pr.PreviousClose * 100
			if divergence > 0.5 || divergence < -0.5 {
				closeLine += fmt.Sprintf("\n  - ⚠ 전일 스냅샷(%s) 종가 %s와 KIS 전일종가 %s 불일치 (%.2f%%) — 스냅샷 저장 시점 오류 가능",
					p.Date, number(prevJSON, 2), number(pr.PreviousClose, 2), divergence)
			}
		}
	}
	b.WriteString("- 일봉 값 (" + pr.Status + "): " + closeLine + "\n")
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
	f := s.Flow
	prevText := func(v *float64) string {
		if v == nil {
			return "N/A"
		}
		return eok(*v)
	}
	b.WriteString(fmt.Sprintf("| 외국인 | %s | %s |\n", eok(f.ForeignEok), prevText(f.ForeignPrevEok)))
	b.WriteString(fmt.Sprintf("| 기관 | %s | %s |\n", eok(f.InstitutionEok), prevText(f.InstitutionPrevEok)))
	b.WriteString(fmt.Sprintf("| 개인 | %s | %s |\n", eok(f.IndividualEok), prevText(f.IndividualPrevEok)))
	b.WriteString(fmt.Sprintf("| 기타법인 | %s | N/A |\n\n", prevText(f.OtherCorporateEok)))
	b.WriteString(fmt.Sprintf("- 당일 기준일: %s · 이전 거래일: %s (%s)\n", f.Date, f.PreviousDate, f.PreviousFlowStatus))
	b.WriteString(fmt.Sprintf("- 3주체 합계: %s억원 · 기타법인 포함 잔차: %s억원\n", signedNumber(f.ResidualEok, 2), valueEokPrecise(f.AllParticipantResidualEok)))
	b.WriteString("- 수급 정합성: `" + f.ReconciliationStatus + "` (출처의 참가자 범위와 반올림 차이 확인 필요)\n\n")
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
	b.WriteString("  - 선물 출처 관측: " + observationLabel(i.FuturesObservedAt) + " · " + i.FuturesStatus + "\n")
	b.WriteString("- 매도 사이드카:\n")
	b.WriteString("  - 발동 가능 상태: `" + i.EligibilityState + "`\n")
	b.WriteString("  - 당일 발동 이력: `TRIGGER_HISTORY_UNKNOWN`\n")
	b.WriteString("  - 사유: `OFFICIAL_EVENT_HISTORY_NOT_COLLECTED`\n\n")
}

func renderGlobal(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 4. 글로벌 참고 시세 (출처별 비교 기준)\n| 자산 | 변동률 | 비교 기준 | 관측 시각 |\n|---|---|---|---|\n")
	for _, item := range []struct{ label, symbol, basis, defaultTime string }{
		{"닛케이225", "^N225", "Yahoo previousClose", "TIMESTAMP_MISSING"},
		{"나스닥 선물", "NQ=F", "Yahoo previousClose (정산가 미검증)", "TIMESTAMP_MISSING"},
		{"WTI 유가", "CL=F", "Yahoo previousClose (정산가 미검증)", "TIMESTAMP_MISSING"},
		{"BTC", "BTC-USD", "Yahoo previousClose (24h 수익률 미검증)", "TIMESTAMP_MISSING"},
		{"USD/KRW", "KRW=X", "Yahoo KRW=X 호가", "TIMESTAMP_MISSING"},
	} {
		value := na(sectionErr(s, "global"))
		timeStr := item.defaultTime
		if s.Global != nil {
			if q, ok := s.Global.Quotes[item.symbol]; ok {
				value = "N/A"
				if ch := quoteChange(q); ch != nil {
					value = percent(*ch)
				}
				timeStr = quoteTime(q)
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
