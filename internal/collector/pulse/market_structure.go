package pulse

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/parse"
	"golang.org/x/sync/errgroup"
)

type marketTimeStock interface {
	MarketTime(context.Context) (*auth.RESTResponse, error)
}

type programTradeStock interface {
	CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error)
}

type vkospiStock interface {
	ResolveVKOSPICode(context.Context, []string) (string, error)
	InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error)
}

type contributionStock interface {
	KOSPIIndexMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
	InquirePrice(context.Context, string) (*auth.RESTResponse, error)
}

func resolveBusinessDate(ctx context.Context, stock marketTimeStock, fallback string) (string, error) {
	resp, err := stock.MarketTime(ctx)
	if err != nil {
		return fallback, err
	}
	if resp == nil || !resp.IsOK() {
		return fallback, fmt.Errorf("market-time invalid business response")
	}
	row := resp.FirstRow("output1")
	if row == nil {
		return fallback, fmt.Errorf("market-time output1 행 없음")
	}
	today := normalizeYMD(valueString(row["today"]))
	latest := ""
	for _, key := range []string{"date1", "date2", "date3", "date4", "date5"} {
		date := normalizeYMD(valueString(row[key]))
		if date == "" {
			continue
		}
		if today != "" && date == today {
			return today, nil
		}
		if (today == "" || date <= today) && date > latest {
			latest = date
		}
	}
	if latest != "" {
		return latest, nil
	}
	if today != "" {
		return today, nil
	}
	return fallback, fmt.Errorf("market-time 영업일 필드 없음")
}

func normalizeYMD(v string) string {
	v = strings.TrimSpace(v)
	if len(v) != 8 {
		return ""
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return v
}

func valueString(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return fmt.Sprint(v)
}

func collectProgramTrade(ctx context.Context, stock programTradeStock, marketClass string) (ProgramTradeSnapshot, error) {
	resp, err := stock.CompProgramTradeToday(ctx, marketClass)
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	if resp == nil || !resp.IsOK() {
		return ProgramTradeSnapshot{}, fmt.Errorf("program: invalid business response")
	}
	rows := resp.Rows("output")
	if len(rows) == 0 {
		return ProgramTradeSnapshot{}, fmt.Errorf("comp-program-trade-today (%s): output 행 없음", marketClass)
	}

	latest := rows[0]
	latestHour := valueString(latest["bsop_hour"])
	for _, row := range rows[1:] {
		hour := valueString(row["bsop_hour"])
		if hour > latestHour {
			latest, latestHour = row, hour
		}
	}
	get := func(key string) (float64, error) {
		v, ok := parse.Num(latest, key)
		if !ok || !finite(v) {
			return 0, fmt.Errorf("%s 필드 없음", key)
		}
		return v / millionToEok, nil
	}
	arbitrage, err := get("arbt_smtn_ntby_tr_pbmn")
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	nonArbitrage, err := get("nabt_smtn_ntby_tr_pbmn")
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	total, err := get("whol_smtn_ntby_tr_pbmn")
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	asOf := latestHour
	return ProgramTradeSnapshot{
		Arbitrage: arbitrage, NonArbitrage: nonArbitrage, Total: total,
		AsOf: asOf, OK: true, LastTS: sourceTimestamp(latest),
	}, nil
}

func collectIndexFuture(ctx context.Context, future DomesticFuture, businessDate, market string, now time.Time) (IndexFutureSnapshot, error) {
	if future == nil {
		return IndexFutureSnapshot{}, fmt.Errorf("domestic future service is nil")
	}
	var code, name string
	if market == "KOSPI200" {
		resolved, err := future.ResolveNearMonthKOSPI200Futures(ctx, businessDate)
		if err != nil {
			return IndexFutureSnapshot{}, err
		}
		code, name = resolved.Record.ShortCode, resolved.Record.Name
	} else {
		resolved, err := future.ResolveNearMonthKOSDAQ150Futures(ctx, businessDate)
		if err != nil {
			return IndexFutureSnapshot{}, err
		}
		code, name = resolved.Record.ShortCode, resolved.Record.Name
	}
	resp, err := future.InquirePrice(ctx, "F", code)
	if err != nil {
		return IndexFutureSnapshot{}, err
	}
	if resp == nil || !resp.IsOK() {
		return IndexFutureSnapshot{}, fmt.Errorf("future: invalid business response")
	}
	futureRow := resp.FirstRow("output1", "output")
	spotRow := resp.FirstRow("output3")
	if futureRow == nil || spotRow == nil {
		return IndexFutureSnapshot{}, fmt.Errorf("index future %s: output1/output3 행 없음", code)
	}
	price, ok := parse.Num(futureRow, "futs_prpr")
	if !ok {
		return IndexFutureSnapshot{}, fmt.Errorf("index future %s: futs_prpr 없음", code)
	}
	if !validFields(futureRow, "futs_prpr", "futs_prdy_clpr", "futs_prdy_ctrt") || price <= 0 {
		return IndexFutureSnapshot{}, fmt.Errorf("future: invalid price/change")
	}
	prevClose, _ := parse.Num(futureRow, "futs_prdy_clpr", "futs_sdpr")
	changePct, _ := parse.Num(futureRow, "futs_prdy_ctrt")
	spotPrice, ok := parse.Num(spotRow, "bstp_nmix_prpr")
	if !ok {
		return IndexFutureSnapshot{}, fmt.Errorf("index future %s: output3 spot 없음", code)
	}
	expectedSpot := "2001"
	if market == "KOSDAQ150" {
		expectedSpot = "3003"
	}
	spotOK := stringField(spotRow, "bstp_cls_code") == expectedSpot && validFields(spotRow, "bstp_nmix_prpr", "bstp_nmix_prdy_ctrt") && spotPrice > 0
	spotChangePct, _ := parse.Num(spotRow, "bstp_nmix_prdy_ctrt")
	computedBasis := price - spotPrice
	marketBasis, marketBasisOK := parse.Num(futureRow, "mrkt_basis")
	basisMatch := spotOK && marketBasisOK && math.Abs(computedBasis-marketBasis) <= 0.05

	lastTS := sourceTimestamp(futureRow)
	freshness, ageSecs, staleReason := DetermineFreshness("KRX", lastTS, now, false)

	return IndexFutureSnapshot{
		Code: code, Name: name, Price: price, PrevClose: prevClose, ChangePct: changePct,
		SpotPrice: spotPrice, SpotChangePct: spotChangePct, Basis: computedBasis, RawSpread: computedBasis,
		MarketBasis: marketBasis, MarketBasisOK: marketBasisOK, BasisMatch: basisMatch, OK: true, SpotOK: spotOK, SpotCode: stringField(spotRow, "bstp_cls_code"), Alignment: "ALIGNMENT_UNVERIFIED",
		LastTS: lastTS, FetchedAt: now, Freshness: freshness, AgeSeconds: ageSecs, StaleReason: staleReason,
	}, nil
}

func collectVKOSPI(ctx context.Context, stock vkospiStock, naverClient NaverFinance, now time.Time) (VolatilitySnapshot, error) {
	var lastErr error
	lastTS := time.Time{}
	freshness, ageSecs, staleReason := DetermineFreshness("KRX", lastTS, now, false)

	if stock != nil {
		code, resolveErr := stock.ResolveVKOSPICode(ctx, nil)
		if resolveErr == nil {
			resp, err := stock.InquireVKOSPIPrice(ctx, code)
			if err == nil && resp != nil && resp.IsOK() {
				if row := resp.FirstRow("output"); row != nil {
					lastTS = sourceTimestamp(row)
					freshness, ageSecs, staleReason = DetermineFreshness("KRX", lastTS, now, false)
					value, valueOK := parse.Num(row, "bstp_nmix_prpr")
					change, changeOK := parse.Num(row, "bstp_nmix_prdy_ctrt")
					if valueOK && changeOK && value >= 5 && value <= 100 {
						return VolatilitySnapshot{
							Code: code, Value: value, ChangePct: change, Source: "KIS", OK: true,
							LastTS: lastTS, FetchedAt: now, Freshness: freshness, AgeSeconds: ageSecs, StaleReason: staleReason,
						}, nil
					}
				}
			} else if err != nil {
				lastErr = err
			}
		} else {
			lastErr = resolveErr
		}
	}

	if naverClient != nil {
		quote, err := naverClient.GetIndexQuote(ctx, "VKOSPI")
		if err == nil && quote != nil && quote.Price >= 5 && quote.Price <= 100 {
			return VolatilitySnapshot{
				Code: "VKOSPI", Value: quote.Price, ChangePct: quote.ChangePercent, Source: "Naver", OK: true,
				LastTS: lastTS, FetchedAt: now, Freshness: freshness, AgeSeconds: ageSecs, StaleReason: staleReason,
			}, nil
		}
	}

	if lastErr != nil {
		return VolatilitySnapshot{}, lastErr
	}
	return VolatilitySnapshot{}, fmt.Errorf("VKOSPI collection unavailable")
}

func collectContributions(ctx context.Context, stock contributionStock, businessDate string, idx IndexLevel) ([]IndexContribution, error) {
	if !idx.OK || idx.PrevClose <= 0 {
		return nil, fmt.Errorf("KOSPI 전일 종가 없음")
	}
	summary, err := stock.KOSPIIndexMarketCapSummary(ctx, businessDate)
	if err != nil {
		return nil, err
	}
	if summary == nil || summary.TotalMarketCap <= 0 {
		return nil, fmt.Errorf("KOSPI 총 시가총액 없음")
	}
	if summary.Universe != domesticstock.KOSPIIndexMasterUniverse || summary.BusinessDate != businessDate {
		return nil, fmt.Errorf("index universe/date unverified")
	}
	total := 0.0
	seen := map[string]bool{}
	for _, item := range summary.Constituents {
		if !item.KOSPIIndexMember || item.PreferredClass != "0" || item.MarketCap <= 0 || !finite(item.MarketCap) || seen[item.Code] {
			return nil, fmt.Errorf("invalid index constituent: %s", item.Code)
		}
		seen[item.Code] = true
		total += item.MarketCap
	}
	if !finite(summary.TotalMarketCap) || math.Abs(total-summary.TotalMarketCap) > 0.01 {
		return nil, fmt.Errorf("index universe denominator mismatch")
	}
	items := append([]domesticstock.KOSPIMarketCapConstituent(nil), summary.Constituents...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].MarketCap == items[j].MarketCap {
			return items[i].Code < items[j].Code
		}
		return items[i].MarketCap > items[j].MarketCap
	})
	limit := 10

	if len(summary.Constituents) < limit {
		limit = len(summary.Constituents)
	}
	out := make([]IndexContribution, 0, limit)
	var failures []string
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for _, item := range items[:limit] {
		item := item
		g.Go(func() error {
			resp, reqErr := stock.InquirePrice(gctx, item.Code)
			if reqErr != nil || resp == nil || !resp.IsOK() {
				mu.Lock()
				failures = append(failures, item.Code)
				mu.Unlock()
				return nil
			}
			row := resp.FirstRow("output")
			changePct, ok := parse.Num(row, "prdy_ctrt")
			if row == nil || !ok || !finite(changePct) {
				mu.Lock()
				failures = append(failures, item.Code)
				mu.Unlock()
				return nil
			}
			weight := item.MarketCap / summary.TotalMarketCap
			contribution := IndexContribution{
				Code: item.Code, Name: item.Name, MarketCap: item.MarketCap,
				WeightPct: weight * 100, ChangePct: changePct,
				PointImpact: nil, Status: "NOT_EVALUATED", WeightStatus: summary.WeightStatus, Universe: summary.Universe, MasterDate: summary.BusinessDate, Denominator: summary.TotalMarketCap, UniverseCount: len(summary.Constituents), SourcePath: summary.SourcePath,
			}
			mu.Lock()
			out = append(out, contribution)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].MarketCap == out[j].MarketCap {
			return out[i].Code < out[j].Code
		}
		return out[i].MarketCap > out[j].MarketCap
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("KOSPI 상위종목 시세 조회 실패: %s", strings.Join(failures, ","))
	}
	if len(failures) > 0 {
		return nil, fmt.Errorf("상위종목 시세 일부 누락 (부분합 미발행): %s", strings.Join(failures, ","))
	}
	return out, nil
}

type OfficialEvent struct {
	BusinessDate string `json:"business_date"`
	Source       string `json:"source"`
	ReleasedAt   string `json:"released_at,omitempty"`
	Market       string `json:"market"`
	Device       string `json:"device"`
	TriggeredAt  string `json:"triggered_at"`
}

func LoadOfficialEvents(date string) []OfficialEvent {
	var events []OfficialEvent

	// Check JSON file
	filePath := os.Getenv("OFFICIAL_EVENTS_FILE")
	if filePath == "" {
		dir := os.Getenv("PULSE_OUTPUT_DIR")
		if dir == "" {
			dir = ".cache/pulse"
		}
		filePath = filepath.Join(dir, fmt.Sprintf("official_events_%s.json", date))
	}

	if f, err := os.Open(filePath); err == nil {
		defer f.Close()
		var fileEvents []OfficialEvent
		if err := json.NewDecoder(f).Decode(&fileEvents); err == nil {
			events = append(events, fileEvents...)
		}
	}

	valid := events[:0]
	for _, ev := range events {
		if ev.BusinessDate == date && strings.HasPrefix(ev.Source, "https://") {
			valid = append(valid, ev)
		}
	}
	return valid
}

func parseHHMMSS(s string, base time.Time) (time.Time, error) {
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, e := time.Parse(layout, s); e == nil {
			return time.Date(base.Year(), base.Month(), base.Day(), t.Hour(), t.Minute(), t.Second(), 0, base.Location()), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid event time")
}

func buildMarketSafety(now time.Time, date string, kospi, kosdaq IndexLevel, k200, kq150 IndexFutureSnapshot, records []PulseRecord) MarketSafety {
	s := MarketSafety{}

	officialEvents := LoadOfficialEvents(date)

	getTriggerTime := func(market, device string) (bool, string, string) {
		for _, ev := range officialEvents {
			if ev.Market == market && ev.Device == device {
				t, e := parseHHMMSS(ev.TriggeredAt, now.In(kstLocation))
				if e == nil && !t.After(now) {
					return true, ev.TriggeredAt, "OFFICIAL"
				}
			}
		}
		return false, "", "UNCONFIRMED"
	}
	applyRelease := func(d *SafetyDeviceStatus) {
		d.State = "TRIGGERED"
		d.ReleasedAt = ""
		for _, ev := range officialEvents {
			if ev.Market == d.Market && ev.Device == d.Device && ev.ReleasedAt != "" {
				t, e := parseHHMMSS(ev.ReleasedAt, now.In(kstLocation))
				trigger, te := parseHHMMSS(d.TriggeredAt, now.In(kstLocation))
				if e == nil && te == nil && t.After(trigger) && !t.After(now) && d.Device != "CB3" {
					d.State = "RELEASED"
					d.ReleasedAt = ev.ReleasedAt
				}
			}
		}
	}
	nowKST := now.In(kstLocation)
	hm := nowKST.Hour()*100 + nowKST.Minute()

	kospiSidecarTriggered := false
	kosdaqSidecarTriggered := false
	var kospiSidecarTriggerTime, kosdaqSidecarTriggerTime string
	var kospiSidecarVer, kosdaqSidecarVer string
	for _, dev := range []string{"SIDECAR_SELL", "SIDECAR_BUY"} {
		if ok, tStr, ver := getTriggerTime("KOSPI", dev); ok {
			kospiSidecarTriggered = true
			kospiSidecarTriggerTime = tStr
			kospiSidecarVer = ver
		}
		if ok, tStr, ver := getTriggerTime("KOSDAQ", dev); ok {
			kosdaqSidecarTriggered = true
			kosdaqSidecarTriggerTime = tStr
			kosdaqSidecarVer = ver
		}
	}

	cbTriggered := map[string]map[string]bool{
		"KOSPI":  {"CB1": false, "CB2": false, "CB3": false},
		"KOSDAQ": {"CB1": false, "CB2": false, "CB3": false},
	}
	cbTriggerTimes := map[string]map[string]string{
		"KOSPI":  {"CB1": "", "CB2": "", "CB3": ""},
		"KOSDAQ": {"CB1": "", "CB2": "", "CB3": ""},
	}
	cbVerifications := map[string]map[string]string{
		"KOSPI":  {"CB1": "", "CB2": "", "CB3": ""},
		"KOSDAQ": {"CB1": "", "CB2": "", "CB3": ""},
	}
	for _, m := range []string{"KOSPI", "KOSDAQ"} {
		for _, dev := range []string{"CB1", "CB2", "CB3"} {
			if ok, tStr, ver := getTriggerTime(m, dev); ok {
				cbTriggered[m][dev] = true
				cbTriggerTimes[m][dev] = tStr
				cbVerifications[m][dev] = ver
			}
		}
	}

	for _, item := range []struct {
		market    string
		f         IndexFutureSnapshot
		threshold float64
		spotTh    float64
		triggered bool
		trigTime  string
		trigVer   string
	}{
		{"KOSPI", k200, 5.0, 0.0, kospiSidecarTriggered, kospiSidecarTriggerTime, kospiSidecarVer},
		{"KOSDAQ", kq150, 6.0, 3.0, kosdaqSidecarTriggered, kosdaqSidecarTriggerTime, kosdaqSidecarVer},
	} {
		if !item.f.OK || (item.spotTh > 0 && !item.f.SpotOK) {
			continue
		}

		for _, dir := range []string{"SELL", "BUY"} {
			device := "SIDECAR_" + dir
			signMult := -1.0
			if dir == "BUY" {
				signMult = 1.0
			}

			devStatus := SafetyDeviceStatus{
				Market:           item.market,
				Device:           device,
				Threshold:        item.threshold,
				FuturesChangePct: ptr(item.f.ChangePct),
			}
			if item.spotTh > 0 {
				devStatus.SpotThreshold = ptr(item.spotTh)
				devStatus.SpotChangePct = ptr(item.f.SpotChangePct)
			}

			hasTriggeredThis := false
			if item.triggered {
				ok, tStr, ver := getTriggerTime(item.market, device)
				if ok {
					hasTriggeredThis = true
					devStatus.TriggeredAt = tStr
					devStatus.Verification = ver
				}
			}

			if hasTriggeredThis {
				devStatus.EligibleNow = false
				devStatus.EligibilityReason = "금일 이미 발동됨 (재발동 불가)"
				applyRelease(&devStatus)
			} else {
				timeEligible := hm >= 905 && hm <= 1450
				noPriorTrigger := !item.triggered

				devStatus.EligibleNow = timeEligible && noPriorTrigger
				if !timeEligible {
					if hm < 905 {
						devStatus.EligibilityReason = "장초반 5분 발동 제한 (09:05 이전)"
						devStatus.State = "NOT_ELIGIBLE"
					} else {
						devStatus.EligibilityReason = "14:50 발동 가능시간 종료"
						devStatus.State = "EXPIRED_FOR_DAY"
					}
				} else if !noPriorTrigger {
					devStatus.EligibilityReason = "금일 다른 사이드카 이미 발동됨 (금일 재발동 불가)"
					devStatus.State = "NOT_ELIGIBLE"
				} else {
					devStatus.EligibilityReason = "발동 가능 시간대"
					devStatus.State = "ELIGIBLE"
				}

				futuresMet := false
				if signMult > 0 {
					futuresMet = item.f.ChangePct >= item.threshold
				} else {
					futuresMet = item.f.ChangePct <= -item.threshold
				}

				spotMet := true
				if item.spotTh > 0 {
					if signMult > 0 {
						spotMet = item.f.SpotChangePct >= item.spotTh
					} else {
						spotMet = item.f.SpotChangePct <= -item.spotTh
					}
				}

				conditionMet := futuresMet && spotMet

				if devStatus.EligibleNow {
					if conditionMet {
						devStatus.State = "CONDITION_OBSERVED"
						devStatus.Verification = "UNCONFIRMED"
						devStatus.ConditionObservedAt = nowKST.Format("15:04:05")
					} else {
						var futuresGap float64
						if signMult > 0 {
							futuresGap = item.threshold - item.f.ChangePct
						} else {
							futuresGap = item.f.ChangePct + item.threshold
						}
						if futuresGap < 0 {
							futuresGap = 0
						}
						devStatus.FuturesGapPct = &futuresGap
						devStatus.ThresholdDistancePct = &futuresGap

						if item.spotTh > 0 {
							var spotGap float64
							if signMult > 0 {
								spotGap = item.spotTh - item.f.SpotChangePct
							} else {
								spotGap = item.f.SpotChangePct + item.spotTh
							}
							if spotGap < 0 {
								spotGap = 0
							}
							devStatus.SpotGapPct = &spotGap
						}
					}
				}
			}

			if devStatus.Verification != "OFFICIAL" && !usableFreshness(item.f.Freshness) {
				devStatus.State = "NOT_EVALUATED"
				devStatus.EligibleNow = false
				devStatus.ConditionObservedAt = ""
				devStatus.ThresholdDistancePct = nil
				devStatus.FuturesGapPct = nil
				devStatus.SpotGapPct = nil
				devStatus.EligibilityReason = "선물 관측 시각 미확인 또는 지연 초과"
			}
			s.Devices = append(s.Devices, devStatus)
		}
	}

	for _, item := range []struct {
		market string
		idx    IndexLevel
	}{
		{"KOSPI", kospi},
		{"KOSDAQ", kosdaq},
	} {
		if !item.idx.OK || item.idx.PrevClose <= 0 {
			continue
		}

		for _, step := range []struct {
			device    string
			threshold float64
			prevCb    string
		}{
			{"CB1", 8.0, ""},
			{"CB2", 15.0, "CB1"},
			{"CB3", 20.0, "CB2"},
		} {
			devStatus := SafetyDeviceStatus{
				Market:         item.market,
				Device:         step.device,
				Threshold:      step.threshold,
				IndexChangePct: ptr(item.idx.ChangePct),
			}

			hasTriggeredThis := cbTriggered[item.market][step.device]
			if hasTriggeredThis {
				devStatus.TriggeredAt = cbTriggerTimes[item.market][step.device]
				devStatus.Verification = cbVerifications[item.market][step.device]
				devStatus.EligibleNow = false
				devStatus.EligibilityReason = "금일 이미 발동됨"
				applyRelease(&devStatus)
			} else {
				timeEligible := false
				if step.device == "CB3" {
					timeEligible = hm >= 900 && hm <= 1530
				} else {
					timeEligible = hm >= 900 && hm <= 1450
				}

				prereqMet := true
				if step.prevCb != "" {
					prevTriggered := cbTriggered[item.market][step.prevCb]
					if !prevTriggered {
						for _, prevDev := range s.Devices {
							if prevDev.Market == item.market && prevDev.Device == step.prevCb {
								if prevDev.State == "TRIGGERED" || prevDev.State == "RELEASED" {
									prevTriggered = true
									break
								}
							}
						}
					}
					prereqMet = prevTriggered
				}

				devStatus.EligibleNow = timeEligible && prereqMet
				if !timeEligible {
					if hm < 900 {
						devStatus.EligibilityReason = "장 시작 전"
						devStatus.State = "NOT_ELIGIBLE"
					} else {
						devStatus.EligibilityReason = "14:50 발동 가능시간 종료"
						devStatus.State = "EXPIRED_FOR_DAY"
					}
				} else if !prereqMet {
					devStatus.EligibilityReason = fmt.Sprintf("선행 CB 단계(%s) 미발동", step.prevCb)
					devStatus.State = "NOT_ELIGIBLE"
				} else {
					devStatus.EligibilityReason = "발동 가능 상태"
					devStatus.State = "ELIGIBLE"
				}

				conditionMet := item.idx.ChangePct <= -step.threshold

				if devStatus.EligibleNow {
					if conditionMet {
						devStatus.State = "CONDITION_OBSERVED"
						devStatus.Verification = "UNCONFIRMED"
						devStatus.ConditionObservedAt = nowKST.Format("15:04:05")
					} else {
						gap := item.idx.ChangePct + step.threshold
						if gap < 0 {
							gap = 0
						}
						devStatus.ThresholdDistancePct = &gap
					}
				}
			}

			if devStatus.Verification != "OFFICIAL" && !usableFreshness(item.idx.Freshness) {
				devStatus.State = "NOT_EVALUATED"
				devStatus.EligibleNow = false
				devStatus.ConditionObservedAt = ""
				devStatus.ThresholdDistancePct = nil
				devStatus.EligibilityReason = "지수 관측 시각 미확인 또는 지연 초과"
			}
			s.Devices = append(s.Devices, devStatus)
		}
	}

	s.CircuitBreakers = buildLegacyCircuitBreakers(kospi, kosdaq)
	s.Sidecars = buildLegacySidecars(k200, kq150, records)

	return s
}

func buildLegacyCircuitBreakers(kospi, kosdaq IndexLevel) []CircuitBreakerStatus {
	var cbs []CircuitBreakerStatus
	for _, item := range []struct {
		name string
		idx  IndexLevel
	}{{"KOSPI", kospi}, {"KOSDAQ", kosdaq}} {
		if !item.idx.OK || item.idx.PrevClose <= 0 || item.idx.Low <= 0 {
			continue
		}
		lowPct := (item.idx.Low - item.idx.PrevClose) / item.idx.PrevClose * 100
		cb := CircuitBreakerStatus{Market: item.name, CurrentChangePct: item.idx.ChangePct, LowChangePct: lowPct, OK: true}
		for _, threshold := range []float64{8, 15, 20} {
			currentGap, currentReached := downsideGap(item.idx.ChangePct, threshold)
			lowGap, lowReached := downsideGap(lowPct, threshold)

			triggerIndexLevel := item.idx.PrevClose * (1.0 - threshold/100.0)
			drawdownRequiredPct := 0.0
			if item.idx.Price > 0 {
				drawdownRequiredPct = (triggerIndexLevel - item.idx.Price) / item.idx.Price * 100.0
			}

			cb.Levels = append(cb.Levels, ThresholdStatus{
				ThresholdPct: threshold, CurrentGapPct: currentGap, LowGapPct: lowGap,
				CurrentReached: currentReached, LowReached: lowReached,
				TriggerIndexLevel: triggerIndexLevel, DrawdownRequiredPct: drawdownRequiredPct,
			})
		}
		cbs = append(cbs, cb)
	}
	return cbs
}

func buildLegacySidecars(k200, kq150 IndexFutureSnapshot, records []PulseRecord) []SidecarStatus {
	var scs []SidecarStatus
	if k200.OK {
		scs = append(scs, buildSidecar("KOSPI", k200, 5, 0, records))
	}
	if kq150.OK && kq150.SpotOK {
		scs = append(scs, buildSidecar("KOSDAQ", kq150, 6, 3, records))
	}
	return scs
}

func downsideGap(changePct, threshold float64) (float64, bool) {
	gap := changePct + threshold
	if gap <= 0 {
		return 0, true
	}
	return gap, false
}

func buildSidecar(market string, f IndexFutureSnapshot, futuresThreshold, spotThreshold float64, records []PulseRecord) SidecarStatus {
	direction := "보합"
	if f.ChangePct > 0 {
		direction = "상승"
	} else if f.ChangePct < 0 {
		direction = "하락"
	}
	futuresGap := math.Max(0, futuresThreshold-math.Abs(f.ChangePct))
	spotGap := 0.0
	spotReached := true
	if spotThreshold > 0 {
		spotGap = math.Max(0, spotThreshold-math.Abs(f.SpotChangePct))
		spotReached = spotGap == 0 && sign(f.ChangePct) == sign(f.SpotChangePct)
	}
	thresholdReached := futuresGap == 0 && spotReached

	status := "NOT_EVALUATED"
	if f.OK && usableFreshness(f.Freshness) && (spotThreshold == 0 || f.SpotOK) {
		status = "CONDITION_NOT_OBSERVED"
		if thresholdReached {
			status = "CONDITION_OBSERVED"
		}
	} else {
		thresholdReached = false
	}
	return SidecarStatus{
		Market: market, FuturesCode: f.Code, Direction: direction,
		FuturesChangePct: f.ChangePct, SpotChangePct: f.SpotChangePct,
		FuturesThresholdPct: futuresThreshold, SpotThresholdPct: spotThreshold,
		FuturesGapPct: futuresGap, SpotGapPct: spotGap,
		ThresholdReached:    thresholdReached,
		ActivationConfirmed: false, OK: true,
		TriggeredToday:     false,
		TriggeredDirection: "",
		Status:             status,
	}
}

func computeProgramDelta(records []PulseRecord, now time.Time, cur ProgramTradeSnapshot, market string) *ProgramTradeDelta {
	if !cur.OK {
		return nil
	}
	prev := LoadNearest(records, now.Add(-time.Hour))
	if prev == nil {
		return nil
	}
	previous := prev.KOSPIProgram
	if market == "kosdaq" {
		previous = prev.KOSDAQProgram
	}
	if !previous.OK || cur.LastTS.IsZero() || !sameDay(cur.LastTS, previous.LastTS) || !previous.LastTS.Before(cur.LastTS) {
		return nil
	}
	return &ProgramTradeDelta{
		RefTS: prev.TS, Elapsed: now.Sub(prev.TS).Minutes(),
		Arbitrage:    cur.Arbitrage - previous.Arbitrage,
		NonArbitrage: cur.NonArbitrage - previous.NonArbitrage,
		Total:        cur.Total - previous.Total,
	}
}

func computeBasisDelta(records []PulseRecord, now time.Time, cur IndexFutureSnapshot, hours int) *BasisDelta {
	if !cur.OK {
		return nil
	}
	prev := LoadNearest(records, now.Add(-time.Duration(hours)*time.Hour))
	if !validRecordTime(prev, now) || !prev.KOSPI200Future.OK || cur.Code != prev.KOSPI200Future.Code || cur.Alignment != "ALIGNED" || prev.KOSPI200Future.Alignment != "ALIGNED" {
		return nil
	}
	return &BasisDelta{RefTS: prev.TS, Elapsed: now.Sub(prev.TS).Minutes(), Value: cur.Basis - prev.KOSPI200Future.Basis}
}
