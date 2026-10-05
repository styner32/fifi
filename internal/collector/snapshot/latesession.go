package snapshot

import (
	"context"
	"fmt"
	"math"
	"time"
)

func collectLateSession(ctx context.Context, deps Deps, date string, price *PriceSection, opts Options) (*LateSessionSection, error) {
	s := &LateSessionSection{BusinessDate: date, Status: StatusInsufficientData, ProgramStatus: "UNAVAILABLE", ProgramReconciledStatus: "NOT_EVALUATED", InstitutionNonArbitrageStatus: "MISSING"}
	if deps.DomesticStock == nil {
		return nil, fmt.Errorf("stock dependency missing")
	}
	if err := fillBasis(ctx, deps, date, s); err != nil {
		s.QualityFlags = append(s.QualityFlags, err.Error())
	}
	if err := fillProgramTradeToday(ctx, deps, s); err != nil {
		s.QualityFlags = append(s.QualityFlags, err.Error())
	}
	if err := fillLateProgramFlow(ctx, deps, s, opts); err != nil {
		s.QualityFlags = append(s.QualityFlags, err.Error())
	}
	if err := fillCloseSessionInvestorFlow(ctx, deps, s); err != nil {
		s.QualityFlags = append(s.QualityFlags, err.Error())
	}
	evaluateLateSessionPatterns(price, s)
	return s, nil
}
func fillBasis(ctx context.Context, deps Deps, date string, s *LateSessionSection) error {
	if deps.DomesticStock == nil || deps.DomesticFuture == nil {
		return fmt.Errorf("basis dependencies missing")
	}
	spot, e := deps.DomesticStock.InquireIndexPrice(ctx, "2001")
	if e != nil {
		return e
	}
	if !spot.IsOK() {
		return fmt.Errorf("spot business error")
	}
	row := firstRow(spot, "output")
	sp, ok := num(row, "bstp_nmix_prpr")
	if !ok || sp <= 0 {
		return fmt.Errorf("invalid spot price")
	}
	contract, e := deps.DomesticFuture.ResolveNearMonthKOSPI200Futures(ctx, date)
	if e != nil {
		return e
	}
	if contract == nil {
		return fmt.Errorf("contract missing")
	}
	s.FuturesContractCode = contract.Record.ShortCode
	s.FuturesContractMonth = contract.Record.MonthClassCode
	s.FuturesStandardCode = contract.Record.StandardCode
	// MonthClassCode is a relative month class; StandardCode is an ISIN, not expiry.
	fut, e := deps.DomesticFuture.InquirePrice(ctx, "F", s.FuturesContractCode)
	if e != nil {
		return e
	}
	if !fut.IsOK() {
		return fmt.Errorf("futures business error")
	}
	fr := firstRow(fut, "output1", "output")
	fp, ok := num(fr, "futs_prpr", "stck_prpr")
	if !ok || fp <= 0 {
		return fmt.Errorf("invalid futures price")
	}
	s.FuturesObservedAt = sourceTime(fr)
	s.SpotObservedAt = sourceTime(row)
	s.FuturesPrice = fp
	s.SpotPrice = sp
	s.BasisPoint = fp - sp
	s.BasisRate = (fp - sp) / sp * 100
	s.BasisOK = true
	s.BasisAlignmentStatus = "RAW_SPREAD / ALIGNMENT_UNVERIFIED"
	s.CrossTimeSpreadStatus = "TIMESTAMP_MISSING"
	if !s.FuturesObservedAt.IsZero() && !s.SpotObservedAt.IsZero() && !s.FuturesObservedAt.Equal(s.SpotObservedAt) {
		s.CrossTimeSpreadStatus = "NOT_A_BASIS / CROSS_TIME_SPREAD"
	}
	// A 15:30 spread additionally requires a dated 15:30 spot observation.
	if s.SpotObservedAt.Format("20060102150405") == date+"153000" {
		if f, e := getFuturesPriceAtTime(ctx, deps.DomesticFuture, s.FuturesContractCode, date, "153000"); e == nil {
			s.Futures1530OK = true
			s.FuturesPrice1530 = f
			s.BasisPoint1530 = f - sp
		}
	}
	return nil
}
func fillProgramTradeToday(ctx context.Context, deps Deps, s *LateSessionSection) error {
	resp, e := deps.DomesticStock.InvestorProgramTradeToday(ctx, "1")
	if e != nil {
		return e
	}
	if !resp.IsOK() {
		return fmt.Errorf("investor program business error")
	}
	seen := map[string]bool{}
	for _, r := range rows(resp, "output1") {
		name := compactName(r["invr_cls_name"])
		code := fmt.Sprint(r["invr_cls_code"])
		if code != "8888" && code != "9100" && name != "기관" && name != "외국인" {
			continue
		}
		if seen[name] {
			return fmt.Errorf("duplicate investor aggregate")
		}
		seen[name] = true
		arb, ao := num(r, "arbt_ntby_amt")
		non, no := num(r, "nabt_ntby_amt")
		total, to := num(r, "all_ntby_amt")
		if !ao || !no || !to || math.Abs(arb+non-total) > 2 {
			continue
		}
		if code == "8888" || name == "기관" {
			s.KOSPINetNonArbitrageOrgan = non / 100
			s.InstitutionOK = true
			s.InstitutionNonArbitrageStatus = "RAW_OBSERVATION"
			s.InstitutionNonArbitrageSemantics = "TIMESTAMP_UNVERIFIED"
		}
		if code == "9100" || name == "외국인" {
			s.KOSPINetNonArbitrageForeign = non / 100
			s.ForeignOK = true
		}
	}
	// Never sum institution aggregates together with their constituent investors.
	return nil
}
func fillLateProgramFlow(ctx context.Context, deps Deps, s *LateSessionSection, opts Options) error {
	resp, e := deps.DomesticStock.CompProgramTradeToday(ctx, "K")
	if e != nil {
		return e
	}
	if !resp.IsOK() {
		return fmt.Errorf("program totals business error")
	}
	var latest map[string]any
	latestHour := ""
	vals := map[string]float64{}
	duplicates := map[string]bool{}
	for _, r := range rows(resp, "output") {
		h, _ := r["bsop_hour"].(string)
		if len(h) != 6 {
			continue
		}
		if _, e := time.Parse("150405", h); e != nil {
			continue
		}
		if d := sourceDate(r); d != "" && d != s.BusinessDate {
			continue
		}
		if h > latestHour {
			latest = r
			latestHour = h
		}
		ts := sourceTime(r)
		v, ok := num(r, "whol_smtn_ntby_tr_pbmn")
		if ok && !ts.IsZero() && ts.Format("20060102") == s.BusinessDate {
			if _, exists := vals[h]; exists {
				duplicates[h] = true
			}
			vals[h] = v
		}
	}
	if latest != nil {
		a, ao := num(latest, "arbt_smtn_ntby_tr_pbmn")
		n, no := num(latest, "nabt_smtn_ntby_tr_pbmn")
		t, to := num(latest, "whol_smtn_ntby_tr_pbmn")
		if ao && no && to && math.Abs(a+n-t) <= 2 {
			s.ProgramOK = true
			s.ProgramSourceTime = latestHour
			s.KOSPINetArbitrageTotal = a / 100
			s.KOSPINetNonArbitrageTotal = n / 100
			s.KOSPIProgramTotalNet = t / 100
			s.OriginalSnapshotArbitrageEok = a / 100
			s.OriginalSnapshotNonArbitrageEok = n / 100
			s.OriginalSnapshotTotalEok = t / 100
			s.ProgramStatus = "RAW_RESPONSE / SOURCE_DATE_UNVERIFIED"
			s.ProgramReconciledStatus = "ARITHMETIC_RECONCILED_ONLY"
		} else {
			s.ProgramStatus = "MISSING_OR_ARITHMETIC_MISMATCH"
		}
	}
	delta := func(a, b string) *float64 {
		av, ao := vals[a]
		bv, bo := vals[b]
		if ao && bo && !duplicates[a] && !duplicates[b] {
			return ptr((bv - av) / 100)
		}
		return nil
	}
	s.LateProgramNetEok = delta("150000", "153000")
	s.CloseSessionProgramNetEok = delta("152000", "153000")
	return nil
}
func fillCloseSessionInvestorFlow(ctx context.Context, deps Deps, s *LateSessionSection) error {
	resp, e := deps.DomesticStock.InquireInvestorTimeByMarket(ctx, "KSP", "0001")
	if e != nil {
		return e
	}
	if !resp.IsOK() {
		return fmt.Errorf("investor time business error")
	}
	vals := map[string][2]float64{}
	duplicate := false
	for _, r := range rows(resp, "output") {
		ts := sourceTime(r)
		if ts.IsZero() || ts.Format("20060102") != s.BusinessDate {
			continue
		}
		h := ts.Format("150405")
		if h != "152000" && h != "153000" {
			continue
		}
		f, fo := num(r, "frgn_ntby_tr_pbmn")
		o, oo := num(r, "orgn_ntby_tr_pbmn")
		if fo && oo {
			if _, exists := vals[h]; exists {
				duplicate = true
			}
			vals[h] = [2]float64{f, o}
		}
	}
	a, ao := vals["152000"]
	b, bo := vals["153000"]
	if ao && bo && !duplicate {
		s.CloseSessionForeignNetEok = ptr((b[0] - a[0]) / 100)
		s.CloseSessionOrganNetEok = ptr((b[1] - a[1]) / 100)
	}
	return nil
}
func evaluateLateSessionPatterns(price *PriceSection, s *LateSessionSection) {
	s.PatternEvaluated = false
	s.PatternDetected = false
	s.PrimaryPattern = "판정 보류"
	s.PatternReason = "INSUFFICIENT_VERIFIED_PATTERN_INPUTS"
	s.Status = StatusInsufficientData
	s.CapitulationScore = nil
	s.ShortSqueezeScore = nil
	s.WindowDressingScore = nil
	s.RebalancingScore = nil
	s.ExpirationArbitrageScore = nil
	s.PatternMissingInputs = map[string][]string{"Late-Session Capitulation": {"DATED_INTERVAL_PRICE_VOLUME_BREADTH"}, "Late-Session Short Squeeze": {"SHORT_VOLUME_BORROW_OPEN_INTEREST"}, "Window Dressing": {"AUCTION_PRICE_VOLUME_PARTICIPANTS"}, "ETF Rebalancing Impact": {"OFFICIAL_REBALANCE_EVENT_ETF_FLOW"}, "Expiration Basis Arbitrage": {"VERIFIED_EXPIRY_SYNCHRONIZED_SPOT_FUTURES"}}
}
func getFuturesPriceAtTime(ctx context.Context, future DomesticFuture, code, date, hour string) (float64, error) {
	resp, e := future.InquireTimeFuopChartPrice(ctx, "F", code, "1", "Y", "N", date, hour)
	if e != nil {
		return 0, e
	}
	if !resp.IsOK() {
		return 0, fmt.Errorf("futures chart business error")
	}
	var found *float64
	for _, r := range rows(resp, "output2") {
		if sourceTime(r).Format("20060102150405") != date+hour {
			continue
		}
		v, ok := num(r, "futs_prpr", "stck_prpr")
		if !ok || v <= 0 {
			return 0, fmt.Errorf("invalid exact futures sample")
		}
		if found != nil {
			return 0, fmt.Errorf("duplicate futures sample")
		}
		found = ptr(v)
	}
	if found == nil {
		return 0, fmt.Errorf("exact dated futures sample missing")
	}
	return *found, nil
}

// Legacy pulse logs contain retrieval times rather than verified source anchors.
func loadProgramValuesFromPulseLogs(dir, date string) (float64, float64, float64, bool, bool, bool) {
	return 0, 0, 0, false, false, false
}

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
