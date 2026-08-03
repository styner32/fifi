package snapshot

import (
	"fmt"
	"strings"
)

// render_extra.go: Section 7 (변동성), 9 (Regime), 10 (집중도) 렌더링

func renderVolatility(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 7. 변동성\n")
	v := s.Volatility
	if v == nil {
		b.WriteString("- " + na("volatility section unavailable") + "\n\n")
		return
	}
	if v.VKOSPI <= 0 {
		b.WriteString("- " + na(v.Reason) + "\n\n")
		return
	}
	vkLine := fmt.Sprintf("%.2f [%s]", v.VKOSPI, v.Level)
	if v.Source != "" {
		vkLine += " · " + v.Source
	}
	if v.VKOSPIChange != 0 {
		vkLine += fmt.Sprintf(" (전일 %s)", percent(v.VKOSPIChange))
	}
	if p != nil && p.Volatility != nil && p.Volatility.VKOSPI > 0 {
		diff := v.VKOSPI - p.Volatility.VKOSPI
		vkLine += fmt.Sprintf("  [%s: %.2f, △%.2f]", p.Date, p.Volatility.VKOSPI, diff)
	}
	b.WriteString("- VKOSPI: " + vkLine + "\n")
	b.WriteString("  - 임계값: <20 정상, 20~25 평상시, 25~30 주의, >30 위험\n")
	if v.VKOSPI5DayAvg > 0 {
		b.WriteString(fmt.Sprintf("- VKOSPI 5일 평균: %.2f\n", v.VKOSPI5DayAvg))
	}
	if v.VIX > 0 {
		vixLine := fmt.Sprintf("%.2f", v.VIX)
		if v.VIXChange != 0 {
			vixLine += fmt.Sprintf(" (전일 %s)", percent(v.VIXChange))
		}
		if p != nil && p.Volatility != nil && p.Volatility.VIX > 0 {
			diff := v.VIX - p.Volatility.VIX
			vixLine += fmt.Sprintf("  [%s: %.2f, △%.2f]", p.Date, p.Volatility.VIX, diff)
		}
		b.WriteString("- VIX: " + vixLine + "\n")
	}
	if v.DecouplingFlag {
		b.WriteString("- 지수-VKOSPI 디커플링: ⚠ 감지됨\n")
	}
	if v.Reason != "" {
		b.WriteString("- 부분 오류: " + v.Reason + "\n")
	}
	b.WriteString("\n")
}

func renderCredit(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 8. 신용잔고 / 반대매매\n")
	c := s.Credit
	if c == nil {
		b.WriteString("- " + na(sectionErr(s, "credit")) + "\n\n")
		return
	}

	if c.Date != "" {
		b.WriteString(fmt.Sprintf("*참고: 증시자금/신용통계 기준일자: %s (T+2 결제시차 집계)*\n", c.Date))
	}

	isKISStale := p != nil && p.Credit != nil && c.Date != "" && p.Credit.Date != "" && c.Date == p.Credit.Date
	isKOFIAStale := p != nil && p.Credit != nil && c.KofiaDate != "" && p.Credit.KofiaDate != "" && c.KofiaDate == p.Credit.KofiaDate

	creditLine := trillionFromEokPlain(c.CreditLoanBalanceEok)
	if p != nil && p.Credit != nil && p.Credit.CreditLoanBalanceEok != 0 {
		diff := c.CreditLoanBalanceEok - p.Credit.CreditLoanBalanceEok
		diffStr := eok(diff) + "억"
		if isKISStale {
			diffStr = "미갱신"
		}
		creditLine += fmt.Sprintf("  [전일 %s, %s]", trillionFromEokPlain(p.Credit.CreditLoanBalanceEok), diffStr)
	}
	b.WriteString("- 신용융자 잔고: " + creditLine + "\n")

	depositLine := trillionFromEokPlain(c.CustomerDepositEok)
	if c.DepositChangeEok != 0 {
		depositLine += fmt.Sprintf(" (전일 대비 %s억)", eok(c.DepositChangeEok))
	}
	if p != nil && p.Credit != nil && p.Credit.CustomerDepositEok != 0 {
		diff := c.CustomerDepositEok - p.Credit.CustomerDepositEok
		diffStr := eok(diff) + "억"
		if isKISStale {
			diffStr = "미갱신"
		}
		depositLine += fmt.Sprintf("  [전일 %s, %s]", trillionFromEokPlain(p.Credit.CustomerDepositEok), diffStr)
	}
	b.WriteString("- 고객예탁금: " + depositLine + "\n")

	if c.FuturesDepositEok != 0 {
		futuresLine := trillionFromEokPlain(c.FuturesDepositEok)
		if p != nil && p.Credit != nil && p.Credit.FuturesDepositEok != 0 {
			diff := c.FuturesDepositEok - p.Credit.FuturesDepositEok
			diffStr := eok(diff) + "억"
			if isKISStale {
				diffStr = "미갱신"
			}
			futuresLine += fmt.Sprintf("  [전일 %s, %s]", trillionFromEokPlain(p.Credit.FuturesDepositEok), diffStr)
		}
		b.WriteString("- 선물예수금: " + futuresLine + "\n")
	}

	// 반대매매 (KOFIA FreeSIS)
	if c.ForcedSellAmountEok > 0 || c.MarginReceivableEok > 0 {
		if c.MarginReceivableEok > 0 {
			marginLine := eokPlain(c.MarginReceivableEok) + "억"
			if p != nil && p.Credit != nil && p.Credit.MarginReceivableEok > 0 {
				diff := c.MarginReceivableEok - p.Credit.MarginReceivableEok
				diffStr := eok(diff)
				if isKOFIAStale {
					diffStr = "미갱신"
				}
				if isKOFIAStale {
					marginLine += fmt.Sprintf("  [전일 %s억, %s]", eokPlain(p.Credit.MarginReceivableEok), diffStr)
				} else {
					marginLine += fmt.Sprintf("  [전일 %s억, %s억]", eokPlain(p.Credit.MarginReceivableEok), diffStr)
				}
			}
			b.WriteString("- 위탁매매 미수금: " + marginLine + "\n")
		}
		forcedLine := eokPlain(c.ForcedSellAmountEok) + "억"
		if p != nil && p.Credit != nil && p.Credit.ForcedSellAmountEok > 0 {
			diff := c.ForcedSellAmountEok - p.Credit.ForcedSellAmountEok
			diffStr := eok(diff)
			if isKOFIAStale {
				diffStr = "미갱신"
			}
			if isKOFIAStale {
				forcedLine += fmt.Sprintf("  [전일 %s억, %s]", eokPlain(p.Credit.ForcedSellAmountEok), diffStr)
			} else {
				forcedLine += fmt.Sprintf("  [전일 %s억, %s억]", eokPlain(p.Credit.ForcedSellAmountEok), diffStr)
			}
		}
		b.WriteString("- 실제 반대매매: " + forcedLine)
		if c.ForcedSellRatioPct > 0 {
			level := forcedSellLevel(c.ForcedSellRatioPct)
			b.WriteString(fmt.Sprintf(" (전일 미수금 대비 %.1f%% %s)", c.ForcedSellRatioPct, level))
		}
		b.WriteString("\n")
	} else if c.Reason != "" {
		b.WriteString("- 반대매매: " + na(c.Reason) + "\n")
	}
	b.WriteString("\n")
}


func forcedSellLevel(pct float64) string {
	switch {
	case pct >= 5.0:
		return "⚠ [높음]"
	case pct >= 3.0:
		return "[주의]"
	default:
		return "[정상]"
	}
}

func renderRegime(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 9. 매크로 채널\n")
	r := s.Regime
	if r == nil {
		b.WriteString("- " + na("regime section unavailable") + "\n\n")
		return
	}
	if r.Reason != "" && r.Phase == "" {
		b.WriteString("- " + na(r.Reason) + "\n\n")
		return
	}
	phaseLine := r.Phase
	if p != nil && p.Regime != nil && p.Regime.Phase != "" && p.Regime.Phase != r.Phase {
		phaseLine += fmt.Sprintf("  [전일: %s → %s]", p.Regime.Phase, r.Phase)
	}
	b.WriteString("- 시장 국면: " + phaseLine + "\n")
	b.WriteString("- 디커플링 강도\n")
	if r.KOSPINASDAQCorr != 0 {
		b.WriteString(fmt.Sprintf("  - KOSPI vs NASDAQ 30일 상관계수: %.2f%s (시각 정렬 적용)\n", r.KOSPINASDAQCorr, corrLevel(r.KOSPINASDAQCorr)))
	}
	if r.KOSPINIKKEICorr != 0 {
		b.WriteString(fmt.Sprintf("  - KOSPI vs NIKKEI 30일 상관계수: %.2f%s (시각 정렬 적용)\n", r.KOSPINIKKEICorr, corrLevel(r.KOSPINIKKEICorr)))
	}
	
	globalRiskLine := fmt.Sprintf("%.1f / 10", r.GlobalRiskAversionIdx)
	if p != nil && p.Regime != nil {
		diff := r.GlobalRiskAversionIdx - p.Regime.GlobalRiskAversionIdx
		if diff != 0 {
			globalRiskLine += fmt.Sprintf("  [전일 %.1f, %s]", p.Regime.GlobalRiskAversionIdx, signedNumber(diff, 1))
		}
	}
	b.WriteString("- 글로벌 위험회피 지수: " + globalRiskLine + "\n")

	domStressLine := fmt.Sprintf("%.1f / 10", r.DomesticMarketStressIdx)
	if p != nil && p.Regime != nil {
		diff := r.DomesticMarketStressIdx - p.Regime.DomesticMarketStressIdx
		if diff != 0 {
			domStressLine += fmt.Sprintf("  [전일 %.1f, %s]", p.Regime.DomesticMarketStressIdx, signedNumber(diff, 1))
		}
	}
	b.WriteString("- 국내 시장 스트레스 지수: " + domStressLine + "\n")

	if r.Reason != "" {
		b.WriteString("- 부분 오류: " + r.Reason + "\n")
	}
	b.WriteString("\n")
}

func corrLevel(corr float64) string {
	switch {
	case corr >= 0.7:
		return " (안정)"
	case corr >= 0.4:
		return " (디커플링 진행)"
	default:
		return " ⚠ (디커플링 심화)"
	}
}

func renderConcentration(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 10. 시장 집중도\n")
	c := s.Concentration
	if c == nil {
		b.WriteString("- " + na("concentration section unavailable") + "\n\n")
		return
	}
	if c.Reason != "" && c.HHI == 0 {
		b.WriteString("- " + na(c.Reason) + "\n\n")
		return
	}

	isStale := p != nil && p.Concentration != nil && c.Date != "" && p.Concentration.Date != "" && c.Date == p.Concentration.Date

	top5Line := fmt.Sprintf("%.1f%%", c.Top5Percent)
	top10Line := fmt.Sprintf("%.1f%%", c.Top10Percent)
	hhiLevelStr := c.HHILevel
	if s.Cumulative != nil && s.Cumulative.SamsungSKHynixCapRatio != nil && *s.Cumulative.SamsungSKHynixCapRatio >= 50.0 {
		hhiLevelStr += " (🔴 상위 2종목 쏠림 극심)"
	}
	hhiLine := fmt.Sprintf("%.0f [%s]", c.HHI, hhiLevelStr)
	if p != nil && p.Concentration != nil {
		if p.Concentration.Top5Percent > 0 {
			diff := c.Top5Percent - p.Concentration.Top5Percent
			diffStr := signedNumber(diff, 1) + "%p"
			if isStale {
				diffStr = "미갱신"
			}
			top5Line += fmt.Sprintf("  [전일 %.1f%%, %s]", p.Concentration.Top5Percent, diffStr)
		}
		if p.Concentration.Top10Percent > 0 {
			diff := c.Top10Percent - p.Concentration.Top10Percent
			diffStr := signedNumber(diff, 1) + "%p"
			if isStale {
				diffStr = "미갱신"
			}
			top10Line += fmt.Sprintf("  [전일 %.1f%%, %s]", p.Concentration.Top10Percent, diffStr)
		}
		if p.Concentration.HHI > 0 {
			diff := c.HHI - p.Concentration.HHI
			diffStr := signedNumber(diff, 0)
			if isStale {
				diffStr = "미갱신"
			}
			hhiLine += fmt.Sprintf("  [전일 %.0f, %s]", p.Concentration.HHI, diffStr)
		}
	}
	b.WriteString("- 상위 5종목 시총 비중: " + top5Line + "\n")
	b.WriteString("- 상위 10종목 시총 비중: " + top10Line + "\n")
	b.WriteString("- HHI (시총 기준): " + hhiLine + "\n")
	b.WriteString("  - 임계값: <1,500 비집중, 1,500~2,500 중간, >2,500 고집중\n")
	b.WriteString("- 상위 종목 집중 위험: " + c.RiskLevel + "\n")
	if s.Cumulative != nil && s.Cumulative.SamsungSKHynixCapRatio != nil {
		b.WriteString(fmt.Sprintf("  - 구성요소: 당일 상위 2종목(삼성전자+SK하이닉스) 비중 %.2f%%, HHI %.0f\n", *s.Cumulative.SamsungSKHynixCapRatio, c.HHI))
	}
	b.WriteString("\n")
}

func renderLateSession(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 11. 막판 수급 및 다중 패턴 분석\n")
	ls := s.LateSession
	if ls == nil {
		b.WriteString("- " + na(sectionErr(s, "late_session")) + "\n\n")
		return
	}

	// 1) 선물-현물 베이시스
	if ls.SpotPrice > 0 {
		closeBasisStr := fmt.Sprintf("%.2fpt (%.2f%%)", ls.BasisPoint, ls.BasisRate)
		if p != nil && p.LateSession != nil && p.LateSession.SpotPrice > 0 {
			diffPoint := ls.BasisPoint - p.LateSession.BasisPoint
			closeBasisStr += fmt.Sprintf(" [전일 %.2fpt, %spt]", p.LateSession.BasisPoint, signedNumber(diffPoint, 2))
		}
		
		sameTimeBasisRate := 0.0
		if ls.SpotPrice > 0 {
			sameTimeBasisRate = (ls.BasisPoint1530 / ls.SpotPrice) * 100
		}
		sameTimeBasisStr := fmt.Sprintf("%.2fpt (%.2f%%)", ls.BasisPoint1530, sameTimeBasisRate)
		alignNote := "판정 보류 — 이론 베이시스 미수집"
		if ls.BasisAlignmentStatus == "ALIGNMENT_UNVERIFIED" {
			alignNote = "판정 보류 — ALIGNMENT_UNVERIFIED"
		}
		
		b.WriteString(fmt.Sprintf("- 선물-현물 동시점 베이시스 (15:30 KST): %s (%s)\n", sameTimeBasisStr, alignNote))
		b.WriteString(fmt.Sprintf("- 선물-현물 종가간 스프레드 (15:45 / 15:30): %s\n", closeBasisStr))
		b.WriteString(fmt.Sprintf("  - 현물(KOSPI 200 종가): %.2f | 선물 15:30가: %.2f | 선물 최종 종가: %.2f\n", ls.SpotPrice, ls.FuturesPrice1530, ls.FuturesPrice))
	} else {
		b.WriteString("- 선물-현물 베이시스: N/A\n")
	}

	// 2) 프로그램 비차익 당일 누적 수급 (억 원)
	reconciledNote := ""
	if ls.ProgramReconciledStatus == "NOT_RECONCILED" {
		reconciledNote = " [NOT_RECONCILED: 기관/계 파싱 실패 또는 수급 불일치]"
	}
	b.WriteString("- 코스피 프로그램 비차익 누적 (억 원):" + reconciledNote + "\n")
	b.WriteString(fmt.Sprintf("  - 외국인: %s | 기관: %s | 전체: %s\n", eok(ls.KOSPINetNonArbitrageForeign), eok(ls.KOSPINetNonArbitrageOrgan), eok(ls.KOSPINetNonArbitrageTotal)))

	// 3) 15시 이후 장 막판 흐름
	progNetStr := "N/A"
	if ls.LateProgramNetEok != nil {
		progNetStr = eok(*ls.LateProgramNetEok)
	}
	b.WriteString("- 장 막판 수급 변화 (15:00 ~ 15:30, 억 원):\n")
	b.WriteString(fmt.Sprintf("  - 프로그램 전체 순매수 변화: %s\n", progNetStr))

	// 4) 종가 동시호가 중 수급 변화 (15:20 ~ 15:30)
	closeProgStr := "N/A"
	if ls.CloseSessionProgramNetEok != nil {
		closeProgStr = eok(*ls.CloseSessionProgramNetEok)
	}
	b.WriteString("- 종가 동시호가 수급 변화 (15:20 ~ 15:30, 억 원):\n")
	b.WriteString(fmt.Sprintf("  - 프로그램 전체 순매수 변화: %s\n", closeProgStr))
	if ls.CloseSessionForeignNetEok != nil || ls.CloseSessionOrganNetEok != nil {
		foreignStr := "N/A"
		if ls.CloseSessionForeignNetEok != nil {
			foreignStr = eok(*ls.CloseSessionForeignNetEok)
		}
		organStr := "N/A"
		if ls.CloseSessionOrganNetEok != nil {
			organStr = eok(*ls.CloseSessionOrganNetEok)
		}
		b.WriteString(fmt.Sprintf("  - 외국인(시장): %s | 기관(시장): %s\n", foreignStr, organStr))
	}

	// 5) 다중 막판 패턴 감지 요약 테이블
	b.WriteString("- **다중 막판 패턴 감지 요약 (Late-Session Pattern Analysis)**:\n\n")

	fmtScore := func(score *float64) string {
		if !ls.PatternEvaluated || score == nil {
			return "`N/A`"
		}
		return fmt.Sprintf("`%.1f`", *score)
	}

	status := func(score *float64, name string) string {
		if !ls.PatternEvaluated || score == nil {
			return "판정 보류"
		}
		if ls.PatternDetected && ls.PrimaryPattern == name {
			switch name {
			case "Late-Session Capitulation":
				return "**🚨 주도 패턴 (LSC)**"
			case "Late-Session Short Squeeze":
				return "**🚀 주도 패턴 (LSS)**"
			case "Window Dressing":
				return "**📈 주도 패턴 (WD)**"
			case "ETF Rebalancing Impact":
				return "**⚖️ 주도 패턴 (ERI)**"
			case "Expiration Basis Arbitrage":
				return "**⚡ 주도 패턴 (EBA)**"
			default:
				return "**🔥 주도 패턴 (Dominant)**"
			}
		}
		if *score >= 2.0 {
			return "⚠️ 감지 (Detected)"
		}
		return "정상 (Normal)"
	}

	b.WriteString("| 패턴명 (Pattern Name) | 감지 점수 (Score) | 임계값 (Threshold) | 판정 상태 (Status) |\n")
	b.WriteString("| :--- | :---: | :---: | :--- |\n")
	b.WriteString(fmt.Sprintf("| 1. **Late-Session Capitulation** (후반 투매) | %s | 2.0 | %s |\n", fmtScore(ls.CapitulationScore), status(ls.CapitulationScore, "Late-Session Capitulation")))
	b.WriteString(fmt.Sprintf("| 2. **Late-Session Short Squeeze** (후반 숏스퀴즈) | %s | 2.0 | %s |\n", fmtScore(ls.ShortSqueezeScore), status(ls.ShortSqueezeScore, "Late-Session Short Squeeze")))
	b.WriteString(fmt.Sprintf("| 3. **Window Dressing** (인위적 종가 관리) | %s | 2.0 | %s |\n", fmtScore(ls.WindowDressingScore), status(ls.WindowDressingScore, "Window Dressing")))
	b.WriteString(fmt.Sprintf("| 4. **ETF Rebalancing Impact** (패시브 리밸런싱) | %s | 2.0 | %s |\n", fmtScore(ls.RebalancingScore), status(ls.RebalancingScore, "ETF Rebalancing Impact")))
	b.WriteString(fmt.Sprintf("| 5. **Expiration Basis Arbitrage** (만기일 차익 청산) | %s | 2.0 | %s |\n", fmtScore(ls.ExpirationArbitrageScore), status(ls.ExpirationArbitrageScore, "Expiration Basis Arbitrage")))
	b.WriteString("\n")

	// 6) 패턴 세부 정보 및 경고 카드
	if ls.PatternDetected {
		b.WriteString("- **패턴 감지 세부 정보**:\n\n")
		switch ls.PrimaryPattern {
		case "Late-Session Capitulation":
			b.WriteString("> [!WARNING]\n")
			b.WriteString(fmt.Sprintf("> **🚨 LATE-SESSION CAPITULATION EVENT DETECTED (감지 점수: %s)**\n", fmtScore(ls.CapitulationScore)))
			b.WriteString("> 장중 고점 대비 반등을 시도했으나 장 막판 동시호가에 외국인 및 비차익 프로그램 매도 폭탄이 쏟아져 당일 저가 부근에서 마감하였습니다. 익일 시초가 반대매매 갭다운에 유의하십시오.\n\n")
		case "Late-Session Short Squeeze":
			b.WriteString("> [!TIP]\n")
			b.WriteString(fmt.Sprintf("> **🚀 LATE-SESSION SHORT SQUEEZE EVENT DETECTED (감지 점수: %s)**\n", fmtScore(ls.ShortSqueezeScore)))
			b.WriteString("> 장 막판 오버나잇 리스크를 회피하려는 숏커버/숏스퀴즈 물량이 집중 유입되며 당일 고가 부근에서 마감하였습니다. 숏 포지션 청산에 따른 매수세가 종가 동시호가까지 강하게 이어졌습니다.\n\n")
		case "Window Dressing":
			b.WriteString("> [!NOTE]\n")
			b.WriteString(fmt.Sprintf("> **📈 WINDOW DRESSING EVENT DETECTED (감지 점수: %s)**\n", fmtScore(ls.WindowDressingScore)))
			b.WriteString("> 분기/반기/연말 영업일 종가 동시호가에 기관투자자의 인위적인 포트폴리오 수익률 관리 매수세가 집중 유입되었습니다. 본질 가치와 무관하게 종가가 왜곡되었을 가능성이 있으므로 익일 정상화(되돌림) 흐름에 주의하십시오.\n\n")
		case "ETF Rebalancing Impact":
			b.WriteString("> [!IMPORTANT]\n")
			b.WriteString(fmt.Sprintf("> **⚖️ ETF REBALANCING IMPACT EVENT DETECTED (감지 점수: %s)**\n", fmtScore(ls.RebalancingScore)))
			b.WriteString("> 주요 지수(MSCI, KRX300 등) 리밸런싱일을 맞이하여 장 마감 동시호가에 대규모 패시브 자금의 기계적 매매(프로그램/외국인)가 대거 집행되었습니다. 단기 수급 왜곡에 따른 일시적 가격 변동성이 극대화되었습니다.\n\n")
		case "Expiration Basis Arbitrage":
			b.WriteString("> [!WARNING]\n")
			b.WriteString(fmt.Sprintf("> **⚡ EXPIRATION BASIS ARBITRAGE EVENT DETECTED (감지 점수: %s)**\n", fmtScore(ls.ExpirationArbitrageScore)))
			b.WriteString("> 선물/옵션 만기일을 맞이하여 선물-현물 괴리(Basis) 청산을 위한 대규모 매수/매도 프로그램 차익거래 물량이 종가 동시호가에 대거 쏟아졌습니다. 청산 방향에 따른 강한 왜곡 및 가격 충격이 감지되었습니다.\n\n")
		default:
			b.WriteString(fmt.Sprintf("- **상태**: ⚠ 패턴 감지됨 (%s, 점수: %s)\n\n", ls.PrimaryPattern, fmtScore(ls.CapitulationScore)))
		}
	} else {
		if !ls.PatternEvaluated || ls.PrimaryPattern == "판정 보류 — 데이터 미수집" {
			reasonStr := "장 막판 또는 동시호가 데이터 수집 누락으로 패턴 평가가 수행되지 않음"
			if ls.PatternReason != "" {
				reasonStr = fmt.Sprintf("사유: %s (데이터 수집 누락)", ls.PatternReason)
			}
			b.WriteString(fmt.Sprintf("- **패턴 감지 세부 정보**: 판정 보류 (%s)\n\n", reasonStr))
		} else {
			b.WriteString("- **패턴 감지 세부 정보**: 정상 상태 (특이 수급 패턴이 감지되지 않음)\n\n")
		}
	}
}
