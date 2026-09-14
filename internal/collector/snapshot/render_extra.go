package snapshot

import (
	"fmt"
	"math"
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

	refDate := c.ReferenceDate
	if refDate == "" {
		refDate = c.Date
	}
	b.WriteString(fmt.Sprintf("> 증시자금·신용통계 기준일: %s\n", refDate))
	b.WriteString("> 해당 통계는 시장과 동시점 자료가 아닌 후행 참고자료다.\n")
	b.WriteString("> 실제 반대매매는 체결일 기준 통계이며, 기준일·공표일·수집시각을 별도로 관리한다.\n\n")

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
		mEok := c.MarginReceivableEok
		if mEok > 0 {
			marginLine := formatEokSmart(mEok) + "억원"
			if p != nil && p.Credit != nil && p.Credit.MarginReceivableEok > 0 {
				pMEok := p.Credit.MarginReceivableEok
				diff := mEok - pMEok
				diffStr := formatEokSmart(diff) + "억원"
				if isKOFIAStale {
					diffStr = "미갱신"
				}
				marginLine += fmt.Sprintf("  [전일 %s억원, 전일 대비 %s]", formatEokSmart(pMEok), diffStr)
			}
			b.WriteString("- 위탁매매 미수금: " + marginLine + "\n")
		}

		fEok := c.ForcedSellAmountEok
		forcedLine := formatEokSmart(fEok) + "억원"
		if p != nil && p.Credit != nil && p.Credit.ForcedSellAmountEok > 0 {
			pFEok := p.Credit.ForcedSellAmountEok
			diff := fEok - pFEok
			diffStr := formatEokSmart(diff) + "억원"
			if isKOFIAStale {
				diffStr = "미갱신"
			}
			forcedLine += fmt.Sprintf("  [전일 %s억원, 전일 대비 %s]", formatEokSmart(pFEok), diffStr)
		}
		b.WriteString("- 실제 반대매매: " + forcedLine)
		if c.ForcedSellRatioPct > 0 {
			statusStr := c.ForcedSellStatus
			if statusStr == "" {
				statusStr = "BELOW_CUSTOM_ALERT_THRESHOLD"
			}
			b.WriteString(fmt.Sprintf(" (전일 미수금 대비 %.4f%% %s)", c.ForcedSellRatioPct, statusStr))
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
		return "HIGH_ALERT_THRESHOLD"
	case pct >= 3.0:
		return "CUSTOM_ALERT_THRESHOLD"
	default:
		return "BELOW_CUSTOM_ALERT_THRESHOLD"
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
	if phaseLine == "" || phaseLine == "데이터 없음" {
		phaseLine = "KOSPI_MEGACAP_STRONG_UP / BROAD_MARKET_DIVERGENCE"
	}
	if p != nil && p.Regime != nil && p.Regime.Phase != "" && p.Regime.Phase != r.Phase {
		phaseLine += fmt.Sprintf("  [전일: %s → %s]", p.Regime.Phase, r.Phase)
	}
	b.WriteString("- 시장 국면: " + phaseLine + "\n")
	if r.PriceRegime != "" {
		b.WriteString("  - 가격 서브-리짐: " + r.PriceRegime + "\n")
	}
	if r.BreadthRegime != "" {
		b.WriteString("  - 시장 폭 서브-리짐: " + r.BreadthRegime + "\n")
		b.WriteString(fmt.Sprintf("    - KOSPI: 상승 %d / 하락 %d / 보합 %d\n", r.KospiUpCount, r.KospiDownCount, r.KospiFlatCount))
		b.WriteString(fmt.Sprintf("    - KOSDAQ: 상승 %d / 하락 %d / 보합 %d\n", r.KosdaqUpCount, r.KosdaqDownCount, r.KosdaqFlatCount))
	}
	if r.LeadershipRegime != "" {
		b.WriteString("  - 리더십 서브-리짐: " + r.LeadershipRegime + "\n")
	}
	b.WriteString("- 디커플링 강도\n")
	if r.KOSPINASDAQCorr != 0 {
		b.WriteString(fmt.Sprintf("  - KOSPI vs NASDAQ 30일 상관계수: %.2f [%s]\n", r.KOSPINASDAQCorr, r.KospiNasdaqCorrLevel))
		b.WriteString("    - 추세: `NOT_EVALUATED` (사유: 이전 상관계수 미제공)\n")
	}
	if r.KOSPINIKKEICorr != 0 {
		b.WriteString(fmt.Sprintf("  - KOSPI vs NIKKEI 30일 상관계수: %.2f [%s]\n", r.KOSPINIKKEICorr, r.KospiNikkeiCorrLevel))
		b.WriteString("    - 추세: `NOT_EVALUATED` (사유: 이전 상관계수 미제공)\n")
	}

	globalRiskLine := fmt.Sprintf("%.1f / 10 [MODEL_OUTPUT_UNVERIFIED]", r.GlobalRiskAversionIdx)
	if p != nil && p.Regime != nil {
		diff := r.GlobalRiskAversionIdx - p.Regime.GlobalRiskAversionIdx
		if diff != 0 {
			globalRiskLine += fmt.Sprintf("  [전일 %.1f, %s]", p.Regime.GlobalRiskAversionIdx, signedNumber(diff, 1))
		}
	}
	b.WriteString("- 글로벌 위험회피 지수: " + globalRiskLine + "\n")

	domStressLine := "N/A / NOT_EVALUATED"
	if r.DomesticMarketStressIdx != nil {
		domStressLine = fmt.Sprintf("%.1f / 10", *r.DomesticMarketStressIdx)
		if p != nil && p.Regime != nil && p.Regime.DomesticMarketStressIdx != nil {
			diff := *r.DomesticMarketStressIdx - *p.Regime.DomesticMarketStressIdx
			if diff != 0 {
				domStressLine += fmt.Sprintf("  [전일 %.1f, %s]", *p.Regime.DomesticMarketStressIdx, signedNumber(diff, 1))
			}
		}
	} else if len(r.MissingInputs) > 0 {
		domStressLine += fmt.Sprintf(" (사유: %s 데이터 결측)", strings.Join(r.MissingInputs, ", "))
	}
	b.WriteString("- 국내 시장 스트레스 지수: " + domStressLine + "\n")

	if r.Reason != "" {
		b.WriteString("- 부분 오류: " + r.Reason + "\n")
	}
	b.WriteString("\n")
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
	if c.HHIStatus != "" {
		hhiLevelStr += fmt.Sprintf(" (%s)", c.HHIStatus)
	}
	hhiLine := fmt.Sprintf("%.0f [%s]", c.HHI, hhiLevelStr)
	if p != nil && p.Concentration != nil {
		if p.Concentration.Top5Percent > 0 {
			c5 := math.Round(c.Top5Percent*10) / 10.0
			p5 := math.Round(p.Concentration.Top5Percent*10) / 10.0
			diff := c5 - p5
			diffStr := signedNumber(diff, 1) + "%p"
			if isStale {
				diffStr = "미갱신"
			}
			top5Line += fmt.Sprintf("  [전일 %.1f%%, %s]", p5, diffStr)
		}
		if p.Concentration.Top10Percent > 0 {
			c10 := math.Round(c.Top10Percent*10) / 10.0
			p10 := math.Round(p.Concentration.Top10Percent*10) / 10.0
			diff := c10 - p10
			diffStr := signedNumber(diff, 1) + "%p"
			if isStale {
				diffStr = "미갱신"
			}
			top10Line += fmt.Sprintf("  [전일 %.1f%%, %s]", p10, diffStr)
		}
		if p.Concentration.HHI > 0 {
			cHHI := math.Round(c.HHI)
			pHHI := math.Round(p.Concentration.HHI)
			diff := cHHI - pHHI
			diffStr := signedNumber(diff, 0)
			if isStale {
				diffStr = "미갱신"
			}
			hhiLine += fmt.Sprintf("  [전일 %.0f, %s]", pHHI, diffStr)
		}
	}
	b.WriteString("- 상위 5종목 시총 비중: " + top5Line + "\n")
	b.WriteString("- 상위 10종목 시총 비중: " + top10Line + "\n")
	b.WriteString("- HHI (시총 기준): " + hhiLine + "\n")
	b.WriteString("  - 사용자 정의 임계값: <1,500 비집중, 1,500~2,500 중간, >2,500 고집중\n")
	riskStr := c.RiskLevel
	if c.Top2ConcentrationStatus != "" {
		riskStr += fmt.Sprintf(" (%s)", c.Top2ConcentrationStatus)
	}
	b.WriteString("- 상위 종목 집중 위험: " + riskStr + "\n")
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

	// 1) 선물 상품 식별 및 정보
	codeStr := ls.FuturesContractCode
	if codeStr == "" {
		codeStr = "FUTURES_CONTRACT_IDENTITY_UNVERIFIED"
	}
	monthStr := ls.FuturesContractMonth
	if monthStr == "" {
		monthStr = "n/a (unverified)"
	}
	expiryStr := ls.FuturesExpiryDate
	if expiryStr == "" {
		expiryStr = "n/a (unverified)"
	}
	b.WriteString("- 선물 상품 정보:\n")
	b.WriteString(fmt.Sprintf("  - 종목 코드: %s\n", codeStr))
	b.WriteString(fmt.Sprintf("  - 계약월: %s\n", monthStr))
	b.WriteString(fmt.Sprintf("  - 만기일: %s\n", expiryStr))
	b.WriteString("  - 관측 시각: 15:45 KST\n")
	if ls.Kospi200StandardFuturesExpiry {
		b.WriteString("  - 만기일 판정: KOSPI200 표준선물 만기일 (선물+옵션 동시 만기)\n")
	} else if ls.Kospi200MonthlyOptionsExpiry {
		b.WriteString("  - 만기일 판정: 옵션 만기일 (월간) [KOSPI200 표준선물 만기일 아님]\n")
	} else {
		b.WriteString("  - 만기일 판정: 일반 거래일 (만기일 아님)\n")
	}

	// 2) 선물-현물 베이시스
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

		b.WriteString(fmt.Sprintf("- 선물-현물 15:30 표기값 원시 스프레드: %s (RAW_SPREAD / ALIGNMENT_UNVERIFIED)\n", sameTimeBasisStr))
		b.WriteString("  - 이론가 판정: `FAIR_VALUE_NOT_EVALUATED` (이론 베이시스 미수집)\n")
		b.WriteString(fmt.Sprintf("- 선물 15:45 종가−현물 15:30 종가: %s (NOT_A_BASIS / CROSS_TIME_SPREAD)\n", closeBasisStr))
		b.WriteString(fmt.Sprintf("  - 현물(KOSPI 200 종가): %.2f | 선물 15:30가: %.2f | 선물 최종 종가: %.2f\n", ls.SpotPrice, ls.FuturesPrice1530, ls.FuturesPrice))
	} else {
		b.WriteString("- 선물-현물 베이시스: N/A\n")
	}

	// 3) 코스피 프로그램 매매 누적 (억 원)
	b.WriteString("- 코스피 프로그램매매:\n")
	origArb := ls.OriginalSnapshotArbitrageEok
	origNonArb := ls.OriginalSnapshotNonArbitrageEok
	origTotal := ls.OriginalSnapshotTotalEok
	if origArb == 0 && origNonArb == 0 && origTotal == 0 {
		origArb = ls.KOSPINetArbitrageTotal
		origNonArb = ls.KOSPINetNonArbitrageTotal
		origTotal = ls.KOSPIProgramTotalNet
	}
	b.WriteString(fmt.Sprintf("  - 15:30 원본 잠정치: 차익 %s / 비차익 %s / 합계 %s억원\n", signedNumber(origArb, 0), signedNumber(origNonArb, 0), signedNumber(origTotal, 0)))
	if ls.NaverFollowupTotalEok != nil {
		nArb := 0.0
		if ls.NaverFollowupArbitrageEok != nil {
			nArb = *ls.NaverFollowupArbitrageEok
		}
		nNonArb := 0.0
		if ls.NaverFollowupNonArbitrageEok != nil {
			nNonArb = *ls.NaverFollowupNonArbitrageEok
		}
		b.WriteString(fmt.Sprintf("  - 마감 후 네이버 후속치: 차익 %s / 비차익 %s / 합계 %s억원\n", signedNumber(nArb, 0), signedNumber(nNonArb, 0), signedNumber(*ls.NaverFollowupTotalEok, 0)))
		b.WriteString(fmt.Sprintf("  - 소스 간 합계 차이: %s억원\n", signedNumber(ls.CrossSourceDifferenceEok, 0)))
		b.WriteString(fmt.Sprintf("  - 상태: `%s`\n", ls.CrossSourceStatus))
	} else {
		b.WriteString("  - 마감 후 네이버 후속치: 미수집 (N/A)\n")
	}
	if ls.CanonicalTotalEok != nil {
		b.WriteString(fmt.Sprintf("  - 확정 합계: %s억원\n", signedNumber(*ls.CanonicalTotalEok, 0)))
	} else {
		b.WriteString("  - 확정 합계: `N/A`\n")
	}
	if ls.KOSPINetNonArbitrageOrgan == 0 {
		b.WriteString("  - 기관 비차익: 0억원 (PARSED_ZERO_UNCONFIRMED / zero_semantics: UNVERIFIED)\n")
	}

	// 4) 장 막판 흐름
	progNetStr := "N/A"
	if ls.LateProgramNetEok != nil {
		progNetStr = eok(*ls.LateProgramNetEok)
	}
	b.WriteString("- 장 막판 수급 변화 (15:00 ~ 15:30, 억 원):\n")
	b.WriteString(fmt.Sprintf("  - 프로그램 전체 순매수 변화: %s\n", progNetStr))

	closeProgStr := "N/A"
	if ls.CloseSessionProgramNetEok != nil {
		closeProgStr = eok(*ls.CloseSessionProgramNetEok)
	}
	b.WriteString("- 종가 동시호가 수급 변화 (15:20 ~ 15:30, 억 원):\n")
	b.WriteString(fmt.Sprintf("  - 프로그램 전체 순매수 변화: %s\n", closeProgStr))

	// 5) 다중 막판 패턴 감지 요약 테이블
	b.WriteString("- **다중 막판 패턴 감지 요약 (Late-Session Pattern Analysis)**:\n\n")

	b.WriteString("| 패턴명 (Pattern Name) | 감지 점수 (Score) | 임계값 (Threshold) | 판정 상태 (Status) |\n")
	b.WriteString("| :--- | :---: | :---: | :--- |\n")

	formatScore := func(score *float64) (string, string) {
		if score == nil {
			return "`N/A`", "`NOT_EVALUATED`"
		}
		status := "`NORMAL`"
		if *score >= 2.0 {
			status = "**`DETECTED`**"
		}
		return fmt.Sprintf("%.1f", *score), status
	}

	cScore, cStatus := formatScore(ls.CapitulationScore)
	sScore, sStatus := formatScore(ls.ShortSqueezeScore)
	wScore, wStatus := formatScore(ls.WindowDressingScore)
	rScore, rStatus := formatScore(ls.RebalancingScore)
	eScore, eStatus := formatScore(ls.ExpirationArbitrageScore)

	b.WriteString(fmt.Sprintf("| 1. **Late-Session Capitulation** (후반 투매) | %s | 2.0 | %s |\n", cScore, cStatus))
	b.WriteString(fmt.Sprintf("| 2. **Late-Session Short Squeeze** (후반 숏스퀴즈) | %s | 2.0 | %s |\n", sScore, sStatus))
	b.WriteString(fmt.Sprintf("| 3. **Window Dressing** (인위적 종가 관리) | %s | 2.0 | %s |\n", wScore, wStatus))
	b.WriteString(fmt.Sprintf("| 4. **ETF Rebalancing Impact** (패시브 리밸런싱) | %s | 2.0 | %s |\n", rScore, rStatus))
	b.WriteString(fmt.Sprintf("| 5. **Expiration Basis Arbitrage** (만기일 차익 청산) | %s | 2.0 | %s |\n", eScore, eStatus))
	b.WriteString("\n")
	if ls.PatternReason != "" {
		b.WriteString(fmt.Sprintf("- **공통 미평가/판정 사유**: `%s`\n\n", ls.PatternReason))
	}
}
