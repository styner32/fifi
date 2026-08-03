package snapshot

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// collectLateSession gathers late-session supply-demand statistics, basis, and checks for capitulation.
func collectLateSession(ctx context.Context, deps Deps, date string, priceSec *PriceSection, opts Options) (*LateSessionSection, error) {
	section := &LateSessionSection{
		BusinessDate: date,
		Status:       StatusValid,
	}

	// 1. 선물-현물 베이시스 계산
	if err := fillBasis(ctx, deps, date, section); err != nil {
		fmt.Printf("[latesession] Warning: failed to fetch basis: %v\n", err)
	}

	// 2. 투자자별 프로그램매매 당일 동향 (코스피)
	if err := fillProgramTradeToday(ctx, deps, section); err != nil {
		fmt.Printf("[latesession] Warning: failed to fetch program trade today: %v\n", err)
	}

	// 3. 시간대별 프로그램 매매 추이 (15:00 vs 15:20 vs 15:30)
	if err := fillLateProgramFlow(ctx, deps, section, opts); err != nil {
		fmt.Printf("[latesession] Warning: failed to fetch late program flow: %v\n", err)
	}

	// 4. 시간대별 투자자 매매 동향 (15:20 vs 15:30)
	if err := fillCloseSessionInvestorFlow(ctx, deps, section); err != nil {
		fmt.Printf("[latesession] Note: close session investor flow not fully available: %v\n", err)
	}

	// 5. 다중 패턴 감지 및 판정
	evaluateLateSessionPatterns(priceSec, section)

	return section, nil
}

func fillBasis(ctx context.Context, deps Deps, date string, sec *LateSessionSection) error {
	// 1) 현물 (KOSPI 200) 가격 조회 (코드: 2001)
	spotResp, err := deps.DomesticStock.InquireIndexPrice(ctx, "2001")
	if err != nil {
		return fmt.Errorf("fetch KOSPI200 spot price: %w", err)
	}
	spotRow := firstRow(spotResp, "output")
	if spotRow == nil {
		return fmt.Errorf("KOSPI200 spot price output is nil")
	}
	spotPrice, ok := num(spotRow, "bstp_nmix_prpr")
	if !ok {
		return fmt.Errorf("KOSPI200 spot price 'bstp_nmix_prpr' missing")
	}

	// 2) 최근월물 선물 코드 조회
	resolvedContract, err := deps.DomesticFuture.ResolveNearMonthKOSPI200Futures(ctx, date)
	if err != nil {
		return fmt.Errorf("resolve near-month futures: %w", err)
	}
	futuresCode := resolvedContract.Record.ShortCode

	// 3) 선물 가격 조회
	futuresResp, err := deps.DomesticFuture.InquirePrice(ctx, "F", futuresCode)
	if err != nil {
		return fmt.Errorf("fetch futures price: %w", err)
	}
	if futuresResp == nil || futuresResp.Body == nil {
		return fmt.Errorf("futures price response body is nil")
	}

	futuresRow := firstRow(futuresResp, "output1", "output")
	if futuresRow == nil {
		futuresRow = futuresResp.Body
	}

	futuresPrice, ok := num(futuresRow, "futs_prpr", "stck_prpr")
	if !ok {
		return fmt.Errorf("futures price 'futs_prpr'/'stck_prpr' missing")
	}

	sec.SpotPrice = spotPrice
	sec.FuturesPrice = futuresPrice
	sec.BasisPoint = futuresPrice - spotPrice
	if spotPrice > 0 {
		sec.BasisRate = (sec.BasisPoint / spotPrice) * 100
	}
	sec.BasisAlignmentStatus = "ALIGNMENT_UNVERIFIED"

	// 4) 15:30 선물 가격 조회 (동시점 베이시스 계산용)
	now := time.Now().In(time.FixedZone("KST", 9*3600))
	if now.Hour() > 15 || (now.Hour() == 15 && now.Minute() >= 30) {
		if fPrice1530, err := getFuturesPriceAtTime(ctx, deps.DomesticFuture, futuresCode, date, "153000"); err == nil && fPrice1530 > 0 {
			sec.FuturesPrice1530 = fPrice1530
			sec.BasisPoint1530 = fPrice1530 - spotPrice
		} else {
			sec.FuturesPrice1530 = futuresPrice
			sec.BasisPoint1530 = futuresPrice - spotPrice
		}
	} else {
		sec.FuturesPrice1530 = futuresPrice
		sec.BasisPoint1530 = futuresPrice - spotPrice
	}

	return nil
}

func fillProgramTradeToday(ctx context.Context, deps Deps, sec *LateSessionSection) error {
	resp, err := deps.DomesticStock.InvestorProgramTradeToday(ctx, "1") // 1: 코스피
	if err != nil {
		return err
	}

	rowsList := rows(resp, "output1")

	var foreignMatched, organMatched, totalMatched bool
	var individualEok float64
	var individualFound bool

	for _, row := range rowsList {
		nameVal, ok := row["invr_cls_name"].(string)
		if !ok {
			continue
		}
		netAmt, ok := num(row, "nabt_ntby_amt") // 비차익 순매수 금액 (백만 원 단위)
		if !ok {
			continue
		}
		netAmtEok := netAmt / 100.0

		name := strings.TrimSpace(nameVal)
		switch {
		case strings.Contains(name, "외국인"):
			sec.KOSPINetNonArbitrageForeign = netAmtEok
			foreignMatched = true
		case strings.Contains(name, "기관"):
			sec.KOSPINetNonArbitrageOrgan = netAmtEok
			organMatched = true
		case strings.Contains(name, "개인") || strings.Contains(name, "일반"):
			individualEok = netAmtEok
			individualFound = true
		case name == "계" || strings.Contains(name, "합계") || strings.Contains(name, "전체"):
			sec.KOSPINetNonArbitrageTotal = netAmtEok
			totalMatched = true
		default:
			fmt.Printf("[latesession] Warning: unmatched invr_cls_name '%s' (nabt_ntby_amt=%.0f)\n", name, netAmt)
		}
	}

	if !organMatched || !totalMatched {
		sec.ProgramReconciledStatus = "NOT_RECONCILED"
	} else {
		sec.ProgramReconciledStatus = "RECONCILED"
	}

	// Fallback: Total 미매칭 시 개별 합산으로 보정
	if !totalMatched && (foreignMatched || organMatched) {
		computed := sec.KOSPINetNonArbitrageForeign + sec.KOSPINetNonArbitrageOrgan
		if individualFound {
			computed += individualEok
		}
		sec.KOSPINetNonArbitrageTotal = computed
		fmt.Printf("[latesession] Warning: '합계'/'전체' row not found, computed total=%.0f from components (foreign=%.0f organ=%.0f individual=%.0f)\n",
			computed, sec.KOSPINetNonArbitrageForeign, sec.KOSPINetNonArbitrageOrgan, individualEok)
	}

	return nil
}

func fillLateProgramFlow(ctx context.Context, deps Deps, sec *LateSessionSection, opts Options) error {
	var p1500, p1520, p1530 float64
	var found1500, found1520, found1530 bool

	// 1) Try loading from pulse logs first
	if opts.PulseStoreDir != "" {
		p1500, p1520, p1530, found1500, found1520, found1530 = loadProgramValuesFromPulseLogs(opts.PulseStoreDir, sec.BusinessDate)
	}

	// 2) Fallback to KIS API if not fully found from logs
	if !found1500 || !found1520 || !found1530 {
		resp, err := deps.DomesticStock.CompProgramTradeToday(ctx, "K") // K: 코스피
		if err == nil {
			rowsList := rows(resp, "output")
			getTimeKey := func(row map[string]any) string {
				for _, k := range []string{"bsop_hour", "stck_cntg_hour", "cntg_hour", "aspr_hour"} {
					if val, ok := row[k].(string); ok && val != "" {
						return strings.TrimSpace(val)
					}
				}
				return ""
			}
			for _, row := range rowsList {
				hourVal := getTimeKey(row)
				if hourVal == "" {
					continue
				}
				val, ok := num(row, "whol_smtn_ntby_tr_pbmn")
				if !ok {
					continue
				}

				if !found1500 && (strings.HasPrefix(hourVal, "1500") || (strings.Compare(hourVal, "150000") >= 0 && strings.Compare(hourVal, "150500") <= 0)) {
					p1500 = val
					found1500 = true
				}
				if !found1520 && (strings.HasPrefix(hourVal, "1520") || (strings.Compare(hourVal, "152000") >= 0 && strings.Compare(hourVal, "152200") <= 0)) {
					p1520 = val
					found1520 = true
				}
				if !found1530 && (strings.HasPrefix(hourVal, "1530") || strings.Compare(hourVal, "153000") >= 0) {
					p1530 = val
					found1530 = true
				}
			}
		}
	}

	if found1500 && found1530 {
		sec.LateProgramNetEok = ptr((p1530 - p1500) / 100.0)
	}
	if found1520 && found1530 {
		sec.CloseSessionProgramNetEok = ptr((p1530 - p1520) / 100.0)
	}

	return nil
}

func fillCloseSessionInvestorFlow(ctx context.Context, deps Deps, sec *LateSessionSection) error {
	resp, err := deps.DomesticStock.InquireInvestorTimeByMarket(ctx, "KSP", "0001")
	if err != nil {
		return err
	}

	rowsList := rows(resp, "output")
	if len(rowsList) < 2 {
		return fmt.Errorf("too few rows for time-based flow (got %d), likely end-of-day summary", len(rowsList))
	}

	var f1520, f1530, o1520, o1530 float64
	var found1520, found1530 bool

	getTimeKey := func(row map[string]any) string {
		for _, k := range []string{"aspr_hour", "bsop_hour", "stck_cntg_hour", "cntg_hour"} {
			if val, ok := row[k].(string); ok && val != "" {
				return strings.TrimSpace(val)
			}
		}
		return ""
	}

	for _, row := range rowsList {
		hourVal := getTimeKey(row)
		if hourVal == "" {
			continue
		}

		frgnAmt, okF := num(row, "frgn_ntby_tr_pbmn")
		orgnAmt, okO := num(row, "orgn_ntby_tr_pbmn")
		if !okF || !okO {
			continue
		}

		if strings.HasPrefix(hourVal, "1520") || (strings.Compare(hourVal, "152000") >= 0 && strings.Compare(hourVal, "152100") <= 0) {
			f1520 = frgnAmt
			o1520 = orgnAmt
			found1520 = true
		}
		if strings.HasPrefix(hourVal, "1530") || strings.Compare(hourVal, "153000") >= 0 {
			f1530 = frgnAmt
			o1530 = orgnAmt
			found1530 = true
		}
	}

	if !found1520 {
		for _, row := range rowsList {
			hourVal := getTimeKey(row)
			if len(hourVal) >= 4 && strings.HasPrefix(hourVal, "152") {
				frgnAmt, okF := num(row, "frgn_ntby_tr_pbmn")
				orgnAmt, okO := num(row, "orgn_ntby_tr_pbmn")
				if okF && okO {
					f1520 = frgnAmt
					o1520 = orgnAmt
					found1520 = true
					break
				}
			}
		}
	}

	if !found1530 && len(rowsList) > 0 {
		frgnAmt, okF := num(rowsList[0], "frgn_ntby_tr_pbmn")
		orgnAmt, okO := num(rowsList[0], "orgn_ntby_tr_pbmn")
		if okF && okO {
			f1530 = frgnAmt
			o1530 = orgnAmt
			found1530 = true
		}
	}

	if found1520 && found1530 {
		sec.CloseSessionForeignNetEok = ptr((f1530 - f1520) / 100.0)
		sec.CloseSessionOrganNetEok = ptr((o1530 - o1520) / 100.0)
	}

	return nil
}

func evaluateLateSessionPatterns(priceSec *PriceSection, sec *LateSessionSection) {
	if priceSec == nil || priceSec.Low <= 0 {
		sec.PrimaryPattern = "판정 보류 — 데이터 미수집"
		sec.PatternDetected = false
		sec.PatternEvaluated = false
		sec.PatternReason = "PRICE_DATA_MISSING"
		sec.Status = StatusInsufficientData
		sec.QualityFlags = []string{"PRICE_DATA_MISSING"}
		return
	}

	if sec.LateProgramNetEok == nil || sec.CloseSessionProgramNetEok == nil || sec.CloseSessionForeignNetEok == nil || sec.CloseSessionOrganNetEok == nil {
		sec.PrimaryPattern = "판정 보류 — 데이터 미수집"
		sec.PatternDetected = false
		sec.PatternEvaluated = false
		sec.PatternReason = "INSUFFICIENT_TIMESTAMPS"
		sec.Status = StatusInsufficientData
		sec.QualityFlags = []string{"LATE_SESSION_DATA_MISSING"}
		return
	}

	sec.PatternEvaluated = true

	// 기본 통계 계산
	volatilityRange := (priceSec.High - priceSec.Low) / priceSec.Low
	closeToLow := (priceSec.Close - priceSec.Low) / priceSec.Low
	closeToHigh := (priceSec.High - priceSec.Close) / priceSec.Low

	// 날짜 파싱 및 판정
	t, err := time.Parse("20060102", sec.BusinessDate)
	var isExpiration, isQuarterEnd, isRebalance bool
	if err == nil {
		isExpiration = checkOptionExpirationDay(t)
		isQuarterEnd = checkQuarterEndDay(t)
		isRebalance = checkRebalancingDay(t)
	}

	// 1. Late-Session Capitulation (LSC) 스코어 계산
	lscScore := 0.0
	if volatilityRange >= 0.01 { lscScore += 0.5 }
	if volatilityRange >= 0.02 { lscScore += 0.5 }
	if closeToLow <= 0.003 { lscScore += 1.0 } else if closeToLow <= 0.006 { lscScore += 0.5 }
	if *sec.LateProgramNetEok <= -300.0 { lscScore += 0.5 }
	if *sec.LateProgramNetEok <= -700.0 { lscScore += 0.5 }
	if *sec.CloseSessionForeignNetEok <= -100.0 { lscScore += 0.5 }
	if *sec.CloseSessionForeignNetEok <= -300.0 { lscScore += 0.5 }
	if *sec.CloseSessionProgramNetEok <= -200.0 { lscScore += 0.5 }
	if *sec.CloseSessionProgramNetEok <= -500.0 { lscScore += 0.5 }
	if sec.BasisPoint < -0.5 { lscScore += 0.5 }

	// 2. Late-Session Short Squeeze (LSS) 스코어 계산
	lssScore := 0.0
	if volatilityRange >= 0.01 { lssScore += 0.5 }
	if volatilityRange >= 0.02 { lssScore += 0.5 }
	if closeToHigh <= 0.003 { lssScore += 1.0 } else if closeToHigh <= 0.006 { lssScore += 0.5 }
	if *sec.LateProgramNetEok >= 300.0 { lssScore += 0.5 }
	if *sec.LateProgramNetEok >= 700.0 { lssScore += 0.5 }
	if *sec.CloseSessionForeignNetEok >= 100.0 { lssScore += 0.5 }
	if *sec.CloseSessionForeignNetEok >= 300.0 { lssScore += 0.5 }
	if *sec.CloseSessionProgramNetEok >= 200.0 { lssScore += 0.5 }
	if *sec.CloseSessionProgramNetEok >= 500.0 { lssScore += 0.5 }
	if sec.BasisPoint > 0.5 { lssScore += 0.5 }

	// 3. Window Dressing (WD) 스코어 계산
	wdScore := 0.0
	if isQuarterEnd { wdScore += 1.5 }
	if *sec.CloseSessionOrganNetEok >= 150.0 { wdScore += 1.0 }
	if *sec.CloseSessionOrganNetEok >= 300.0 { wdScore += 1.0 }
	if closeToHigh <= 0.008 { wdScore += 0.5 }

	// 4. ETF Rebalancing Impact (ERI) 스코어 계산
	eriScore := 0.0
	if isRebalance || isQuarterEnd { eriScore += 1.0 }
	absProg := math.Abs(*sec.CloseSessionProgramNetEok)
	if absProg >= 400.0 { eriScore += 1.5 }
	if absProg >= 800.0 { eriScore += 1.5 }
	if math.Abs(*sec.CloseSessionForeignNetEok) >= 300.0 { eriScore += 1.0 }

	// 5. Expiration Basis Arbitrage (EBA) 스코어 계산
	ebaScore := 0.0
	if isExpiration {
		if math.Abs(*sec.CloseSessionProgramNetEok) >= 100.0 {
			absBasis := math.Abs(sec.BasisPoint)
			if absBasis >= 1.0 { ebaScore += 1.0 }
			if absBasis >= 2.0 { ebaScore += 1.0 }

			if math.Abs(*sec.CloseSessionProgramNetEok) >= 300.0 {
				ebaScore += 1.0
			}
			if math.Abs(*sec.CloseSessionProgramNetEok) >= 500.0 {
				ebaScore += 1.0
			}

			if ebaScore > 0 {
				ebaScore += 1.0
			}
		}
	}

	// 각 스코어 저장
	sec.CapitulationScore = ptr(lscScore)
	sec.ShortSqueezeScore = ptr(lssScore)
	sec.WindowDressingScore = ptr(wdScore)
	sec.RebalancingScore = ptr(eriScore)
	sec.ExpirationArbitrageScore = ptr(ebaScore)

	// 최대 점수 비교를 통한 지배 패턴 선정
	maxScore := 0.0
	pattern := "Normal (정상)"

	if lscScore > maxScore { maxScore = lscScore; pattern = "Late-Session Capitulation" }
	if lssScore > maxScore { maxScore = lssScore; pattern = "Late-Session Short Squeeze" }
	if wdScore > maxScore { maxScore = wdScore; pattern = "Window Dressing" }
	if eriScore > maxScore { maxScore = eriScore; pattern = "ETF Rebalancing Impact" }
	if ebaScore > maxScore { maxScore = ebaScore; pattern = "Expiration Basis Arbitrage" }

	if maxScore >= 2.0 {
		sec.PrimaryPattern = pattern
		sec.PatternDetected = true
	} else {
		sec.PrimaryPattern = "Normal (정상)"
		sec.PatternDetected = false
	}
}

// checkOptionExpirationDay 검증: 매월 둘째 목요일인지 판단 (선물/옵션 만기일)
func checkOptionExpirationDay(t time.Time) bool {
	if t.Weekday() != time.Thursday {
		return false
	}
	// 둘째 목요일은 날짜 범위가 무조건 8~14일 사이임
	return t.Day() >= 8 && t.Day() <= 14
}

// checkQuarterEndDay 검증: 3/6/9/12월 말 영업일 부근 (26~31일)
func checkQuarterEndDay(t time.Time) bool {
	m := t.Month()
	if m != time.March && m != time.June && m != time.September && m != time.December {
		return false
	}
	return t.Day() >= 26
}

// checkRebalancingDay 검증: 2/5/8/11월 말일 부근 (패시브 리밸런싱은 분기말/반기말 부근에 자주 집중됨)
func checkRebalancingDay(t time.Time) bool {
	m := t.Month()
	if m != time.February && m != time.May && m != time.August && m != time.November {
		return false
	}
	return t.Day() >= 25
}

func getFuturesPriceAtTime(ctx context.Context, future DomesticFuture, code, date, targetHour string) (float64, error) {
	resp, err := future.InquireTimeFuopChartPrice(ctx, "F", code, "1", "Y", "N", date, targetHour)
	if err != nil {
		return 0, err
	}
	rows := resp.Rows("output2")
	if len(rows) == 0 {
		return 0, fmt.Errorf("no chart rows returned")
	}
	for _, row := range rows {
		hour, _ := row["stck_cntg_hour"].(string)
		hour = strings.TrimSpace(hour)
		if strings.HasPrefix(hour, targetHour[:4]) {
			val, ok := num(row, "futs_prpr", "stck_prpr")
			if ok {
				return val, nil
			}
		}
	}
	val, ok := num(rows[0], "futs_prpr", "stck_prpr")
	if ok {
		return val, nil
	}
	return 0, fmt.Errorf("failed to parse futures price from chart")
}

func loadProgramValuesFromPulseLogs(dir, date string) (p1500, p1520, p1530 float64, f1500, f1520, f1530 bool) {
	if dir == "" {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("pulse_%s.jsonl", date))
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var records []struct {
		T time.Time
		V float64
	}

	kst := time.FixedZone("KST", 9*3600)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec struct {
			TS           time.Time `json:"ts"`
			KOSPIProgram struct {
				Total float64 `json:"total"`
				OK    bool    `json:"ok"`
			} `json:"kospi_program"`
		}
		if err := json.Unmarshal(sc.Bytes(), &rec); err == nil && rec.KOSPIProgram.OK {
			records = append(records, struct {
				T time.Time
				V float64
			}{rec.TS.In(kst), rec.KOSPIProgram.Total})
		}
	}

	findClosest := func(targetHour, targetMin int) (float64, bool) {
		var bestVal float64
		var bestDiff time.Duration = 999 * time.Hour
		for _, r := range records {
			targetTime := time.Date(r.T.Year(), r.T.Month(), r.T.Day(), targetHour, targetMin, 0, 0, r.T.Location())
			diff := r.T.Sub(targetTime)
			if diff < 0 {
				diff = -diff
			}
			if diff < bestDiff && diff <= 10*time.Minute {
				bestDiff = diff
				bestVal = r.V
			}
		}
		return bestVal, bestDiff <= 10*time.Minute
	}

	p1500, f1500 = findClosest(15, 0)
	p1520, f1520 = findClosest(15, 20)
	p1530, f1530 = findClosest(15, 30)
	return
}

