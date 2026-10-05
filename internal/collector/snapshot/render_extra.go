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
		b.WriteString("- N/A\n\n")
		return
	}
	if v.VKOSPI > 0 {
		b.WriteString(fmt.Sprintf("- VKOSPI: %.2f · %s · 기준일 %s · %s · 관측 %s\n", v.VKOSPI, v.Source, missingText(v.Date), v.Status, observationLabel(v.ObservedAt)))
		if v.VKOSPIChangeOK {
			b.WriteString(fmt.Sprintf("  - 출처 일간 등락: %s\n", percent(v.VKOSPIChange)))
		}
	} else {
		b.WriteString("- VKOSPI: N/A · " + v.Reason + "\n")
	}
	if len(v.AverageDates) == 5 {
		b.WriteString(fmt.Sprintf("- VKOSPI 5거래일 평균: %.2f (%s)\n", v.VKOSPI5DayAvg, strings.Join(v.AverageDates, ", ")))
	} else {
		b.WriteString("- VKOSPI 5거래일 평균: N/A / FIVE_DATED_ROWS_REQUIRED\n")
	}
	if v.VIX > 0 {
		b.WriteString(fmt.Sprintf("- VIX: %.2f · %s · %s\n", v.VIX, v.VIXStatus, observationLabel(v.VIXObservedAt)))
	} else {
		b.WriteString("- VIX: N/A\n")
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
	b.WriteString(fmt.Sprintf("> KIS 증시자금 기준일: %s\n", refDate))
	b.WriteString("> 해당 통계는 시장과 동시점 자료가 아닌 후행 참고자료다.\n")
	b.WriteString(fmt.Sprintf("> KOFIA 기준일: %s · 공표 시각: 미제공\n> 예탁금 전일 증감 검산: %s\n\n", c.KofiaDate, c.DepositReconciliationStatus))

	isKISStale := p != nil && p.Credit != nil && c.Date != "" && p.Credit.Date != "" && c.Date == p.Credit.Date
	isKOFIAStale := p != nil && p.Credit != nil && c.KofiaDate != "" && p.Credit.KofiaDate != "" && c.KofiaDate == p.Credit.KofiaDate

	creditLine := trillionFromEokPlain(c.CreditLoanBalanceEok)
	if p != nil && p.Credit != nil && p.Credit.CreditLoanBalanceEok != 0 {
		diff := c.CreditLoanBalanceEok - p.Credit.CreditLoanBalanceEok
		diffStr := eok(diff) + "억"
		if isKISStale {
			diffStr = "미갱신"
		}
		creditLine += fmt.Sprintf("  [이전 저장본 %s, %s]", trillionFromEokPlain(p.Credit.CreditLoanBalanceEok), diffStr)
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
		depositLine += fmt.Sprintf("  [이전 저장본 %s, %s]", trillionFromEokPlain(p.Credit.CustomerDepositEok), diffStr)
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
			futuresLine += fmt.Sprintf("  [이전 저장본 %s, %s]", trillionFromEokPlain(p.Credit.FuturesDepositEok), diffStr)
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
				marginLine += fmt.Sprintf("  [이전 저장본 %s억원, 차이 %s]", formatEokSmart(pMEok), diffStr)
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
			forcedLine += fmt.Sprintf("  [이전 저장본 %s억원, 차이 %s]", formatEokSmart(pFEok), diffStr)
		}
		b.WriteString("- 실제 반대매매: " + forcedLine)
		if c.ForcedSellRatioPct > 0 {
			statusStr := c.ForcedSellStatus
			if statusStr == "" {
				statusStr = "BELOW_CUSTOM_ALERT_THRESHOLD"
			}
			b.WriteString(fmt.Sprintf(" (출처 보고 비율 %.1f%% · %s)", c.ForcedSellRatioPct, statusStr))
		}
		b.WriteString("\n")
	} else {
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
	b.WriteString("## 9. 시장 국면·모델 평가\n")
	if s.Regime != nil {
		b.WriteString("- 가격 방향: " + s.Regime.PriceRegime + "\n")
	}
	b.WriteString("- 시장 간 상관계수: N/A / NOT_EVALUATED (거래일·완결 시각 정렬 미검증)\n")
	b.WriteString("- 글로벌 위험회피·국내 스트레스 점수: N/A / NOT_EVALUATED (검증된 전체 입력 미확보)\n\n")
}

func renderConcentration(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 10. 시장 집중도 (KIS 마스터 참고치)\n")
	c := s.Concentration
	if c == nil {
		b.WriteString("- N/A / dated index universe unavailable\n\n")
		return
	}
	if c.Status != StatusEstimated {
		b.WriteString("- N/A / " + c.Reason + "\n\n")
		return
	}
	b.WriteString(fmt.Sprintf("- 마스터 영업일: %s · 대상 %d종목 · 전체 분모 %s억원 (전일 시총 필드)\n", c.Date, c.UniverseCount, number(c.DenominatorEok, 0)))
	b.WriteString(fmt.Sprintf("- 상위 2 / 5 / 10종목 비중: %.2f%% / %.2f%% / %.2f%%\n", c.Top2Percent, c.Top5Percent, c.Top10Percent))
	b.WriteString(fmt.Sprintf("- HHI: %.2f · %s (%s)\n", c.HHI, c.HHILevel, c.HHIStatus))
	b.WriteString("- 상위 2종목 집중도: " + c.Top2ConcentrationStatus + " (사용자 정의 기준 >45%)\n")
	b.WriteString("- " + c.WeightStatus + " · KRX 공식 지수 가중치 및 포인트 기여도는 미검증\n\n")
}

func renderLateSession(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 11. 막판 수급 및 다중 패턴 분석\n")
	ls := s.LateSession
	if ls == nil {
		b.WriteString("- " + na(sectionErr(s, "late_session")) + "\n\n")
		return
	}

	b.WriteString(fmt.Sprintf("- 선물 코드: %s · 표준코드(ISIN): %s · 최근월 분류: %s\n", ls.FuturesContractCode, ls.FuturesStandardCode, ls.FuturesContractMonth))
	b.WriteString("- 만기일: N/A / CONTRACT_EXPIRY_UNVERIFIED\n")
	b.WriteString("- 선물 출처 관측: " + observationLabel(ls.FuturesObservedAt) + " · 현물 출처 관측: " + observationLabel(ls.SpotObservedAt) + "\n")
	if ls.BasisOK {
		b.WriteString(fmt.Sprintf("- 선물 %.2f − 현물(KOSPI200) %.2f = %.2fp (%.2f%%) · RAW_SPREAD / ALIGNMENT_UNVERIFIED\n", ls.FuturesPrice, ls.SpotPrice, ls.BasisPoint, ls.BasisRate))
	} else {
		b.WriteString("- 선물·현물 원시 차이: N/A\n")
	}
	if ls.Futures1530OK {
		b.WriteString(fmt.Sprintf("- 날짜·15:30 표본 일치 원시 차이: %.2fp (FAIR_VALUE_NOT_EVALUATED)\n", ls.BasisPoint1530))
	} else {
		b.WriteString("- 15:30 동시점 원시 차이: N/A / EXACT_DATED_SAMPLE_MISSING\n")
	}
	if ls.ProgramOK {
		b.WriteString(fmt.Sprintf("- 프로그램 전체시장 응답: 차익 %s / 비차익 %s / 합계 %s억원\n", signedNumber(ls.KOSPINetArbitrageTotal, 2), signedNumber(ls.KOSPINetNonArbitrageTotal, 2), signedNumber(ls.KOSPIProgramTotalNet, 2)))
	} else {
		b.WriteString("- 프로그램 전체시장 응답: N/A\n")
	}
	b.WriteString("  - 출처 시간 필드: " + ls.ProgramSourceTime + " · " + ls.ProgramStatus + " · " + ls.ProgramReconciledStatus + "\n")
	if ls.InstitutionOK {
		b.WriteString(fmt.Sprintf("- 기관 비차익 응답: %s억원 (기관 합계 행; 관측 시각 미검증)\n", signedNumber(ls.KOSPINetNonArbitrageOrgan, 2)))
	} else {
		b.WriteString("- 기관 비차익: N/A / MISSING\n")
	}
	b.WriteString("- 확정 프로그램 합계: N/A / SOURCE_TIME_SCOPE_NOT_RECONCILED\n")

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
