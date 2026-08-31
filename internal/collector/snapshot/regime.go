package snapshot

import (
	"context"
	"math"
	"time"

	"github.com/fifi/internal/external/yahoo"
)

// RegimeSection은 매크로 채널/시장 국면 분류를 담습니다.
type RegimeSection struct {
	Phase                    string        `json:"phase"`
	PriceRegime              string        `json:"price_regime,omitempty"`
	BreadthRegime            string        `json:"breadth_regime,omitempty"`
	CrossMarketRegime        string        `json:"cross_market_regime,omitempty"`
	LeadershipRegime         string        `json:"leadership_regime,omitempty"`
	ProgramRegime            string        `json:"program_regime,omitempty"`
	KOSPINASDAQCorr          float64       `json:"kospi_nasdaq_corr"`
	KOSPINIKKEICorr          float64       `json:"kospi_nikkei_corr"`
	KospiNasdaqCorrLevel     string        `json:"kospi_nasdaq_corr_level,omitempty"`
	KospiNikkeiCorrLevel     string        `json:"kospi_nikkei_corr_level,omitempty"`
	KospiNasdaqCorrTrend     string        `json:"kospi_nasdaq_corr_trend,omitempty"`
	KospiNikkeiCorrTrend     string        `json:"kospi_nikkei_corr_trend,omitempty"`
	GlobalRiskAversionIdx    float64       `json:"global_risk_aversion_idx"` // 글로벌 위험회피지수 0~10
	GlobalRiskAversionStatus string        `json:"global_risk_aversion_status,omitempty"`
	DomesticMarketStressIdx  *float64      `json:"domestic_market_stress_idx"` // 국내 시장 스트레스 지수 (nullable)
	DomesticStressStatus     string        `json:"domestic_stress_status,omitempty"`
	MissingInputs            []string      `json:"missing_inputs,omitempty"`
	KospiUpCount             int           `json:"kospi_up_count,omitempty"`
	KospiDownCount           int           `json:"kospi_down_count,omitempty"`
	KospiFlatCount           int           `json:"kospi_flat_count,omitempty"`
	KosdaqUpCount            int           `json:"kosdaq_up_count,omitempty"`
	KosdaqDownCount          int           `json:"kosdaq_down_count,omitempty"`
	KosdaqFlatCount          int           `json:"kosdaq_flat_count,omitempty"`
	LastValidVKOSPIValue     float64       `json:"last_valid_vkospi_value,omitempty"`
	LastValidVKOSPITime      string        `json:"last_valid_vkospi_time,omitempty"`
	LastValidVKOSPIStatus    string        `json:"last_valid_vkospi_status,omitempty"`
	Reason                   string        `json:"-"`
	Status                   QualityStatus `json:"status,omitempty"`
	QualityFlags             []string      `json:"quality_flags,omitempty"`
}

func correlationLevel(corr float64) string {
	switch {
	case corr >= 0.8:
		return "HIGH_POSITIVE"
	case corr >= 0.5:
		return "MODERATE_POSITIVE"
	case corr >= 0.2:
		return "LOW_POSITIVE"
	case corr >= -0.2:
		return "NEUTRAL"
	default:
		return "NEGATIVE"
	}
}

// collectRegime은 Yahoo 30일 히스토리로 상관계수와 시장 국면을 계산합니다.
func collectRegime(ctx context.Context, yClient YahooQuotes, price *PriceSection, volatility *VolatilitySection, impact *ImpactSection, macro *MacroSection, credit *CreditSection) *RegimeSection {
	s := &RegimeSection{
		Status: StatusValid,
	}
	if volatility != nil && volatility.VKOSPI > 0 {
		s.LastValidVKOSPIValue = volatility.VKOSPI
		s.LastValidVKOSPITime = volatility.ObservedAt.Format(time.RFC3339)
		s.LastValidVKOSPIStatus = string(volatility.Status)
	}

	if yClient == nil {
		s.Reason = "yahoo dependency is nil"
		s.Status = StatusUnavailable
		return s
	}

	// 30일 일별 종가 조회
	symbols := []string{"^KS11", "NQ=F", "^N225", "CL=F", "KRW=X"}
	charts := map[string][]yahoo.DailyClose{}
	var firstErr string
	for _, sym := range symbols {
		closes, err := yClient.GetChartHistory(ctx, sym, "1mo", "1d")
		if err != nil || len(closes) < 5 {
			if firstErr == "" && err != nil {
				firstErr = sym + ": " + err.Error()
			}
			continue
		}
		charts[sym] = closes
	}

	if len(charts) == 0 {
		s.Reason = "chart history unavailable: " + firstErr
		s.Status = StatusUnavailable
	}

	coverage := float64(len(charts)) / float64(len(symbols))
	if coverage < 0.7 {
		s.Status = StatusInsufficientData
		s.QualityFlags = []string{"REGIME_DATA_MISSING"}
	}

	// 상관계수 계산을 위한 데이터 정렬
	var alignedKS, alignedNQ []float64
	var alignedKSN225, alignedN225 []float64

	if ksCloses, ok := charts["^KS11"]; ok {
		if nqCloses, ok2 := charts["NQ=F"]; ok2 {
			alignedKS, alignedNQ = alignUSSeries(ksCloses, nqCloses)
			if len(alignedKS) >= 3 {
				s.KOSPINASDAQCorr = pearsonCorr(alignedKS, alignedNQ)
			}
		}
		if n225Closes, ok2 := charts["^N225"]; ok2 {
			alignedKSN225, alignedN225 = alignNikkeiSeries(ksCloses, n225Closes)
			if len(alignedKSN225) >= 3 {
				s.KOSPINIKKEICorr = pearsonCorr(alignedKSN225, alignedN225)
			}
		}
	}

	s.KospiNasdaqCorrLevel = correlationLevel(s.KOSPINASDAQCorr)
	s.KospiNikkeiCorrLevel = correlationLevel(s.KOSPINIKKEICorr)
	s.KospiNasdaqCorrTrend = string(StatusNotEvaluated)
	s.KospiNikkeiCorrTrend = string(StatusNotEvaluated)

	// 세부 서브-리짐 설정
	s.Phase = classifyPhase(price, impact, volatility, macro)
	if price != nil && price.PreviousClose > 0 && price.Close > price.PreviousClose {
		if price.High > price.Low {
			pos := (price.Close - price.Low) / (price.High - price.Low)
			if pos < 0.8 {
				s.PriceRegime = "KOSPI_STRONG_UP_OFF_HIGH"
			} else {
				s.PriceRegime = "KOSPI_STRONG_UP"
			}
		} else {
			s.PriceRegime = "KOSPI_STRONG_UP"
		}
	} else if price != nil && price.Close > 0 {
		s.PriceRegime = "KOSPI_WEAK"
	}

	// 글로벌 위험회피지수 (Global Risk Aversion Index, 0~10)
	globalScore := 0.0

	// 1. 나스닥 상관성 컴포넌트 (Weight 0.20)
	globalScore += (1.0 - math.Max(-1.0, math.Min(1.0, s.KOSPINASDAQCorr))) / 2.0 * 10.0 * 0.20

	// 2. VIX 컴포넌트 (Weight 0.30)
	vixVal := 15.0
	if volatility != nil && volatility.VIX > 0 {
		vixVal = volatility.VIX
	}
	globalScore += math.Min(vixVal/30.0, 1.0) * 10.0 * 0.30

	// 3. 환율 변동 컴포넌트 (Weight 0.20)
	krwChg := 0.0
	if macro != nil {
		if q, ok := macro.Quotes["KRW=X"]; ok {
			krwChg = math.Abs(q.ChangePercent)
		}
	}
	globalScore += math.Min(krwChg/1.5, 1.0) * 10.0 * 0.20

	// 4. WTI 유가 변동 컴포넌트 (Weight 0.15)
	wtiChg := 0.0
	if macro != nil {
		if q, ok := macro.Quotes["CL=F"]; ok {
			wtiChg = math.Abs(q.ChangePercent)
		}
	}
	globalScore += math.Min(wtiChg/3.0, 1.0) * 10.0 * 0.15

	// 5. 미국 10년물 국채금리 변동 컴포넌트 (Weight 0.15)
	tnxChg := 0.0
	if macro != nil {
		if q, ok := macro.Quotes["^TNX"]; ok {
			tnxChg = math.Abs(q.ChangePercent)
		}
	}
	globalScore += math.Min(tnxChg/3.0, 1.0) * 10.0 * 0.15

	s.GlobalRiskAversionIdx = math.Round(globalScore*10) / 10

	// 국내 시장 스트레스 지수 계산
	var missingInputs []string
	if volatility == nil || volatility.VKOSPI <= 0 {
		missingInputs = append(missingInputs, "VKOSPI_CLOSE")
	}
	if impact == nil || impact.SidecarStatus == "" || impact.SidecarStatus == "unknown" || impact.SidecarStatus == "n/a" {
		missingInputs = append(missingInputs, "OFFICIAL_SIDECAR_HISTORY")
	}
	if macro == nil || macro.Quotes["^TNX"].Price <= 0 {
		missingInputs = append(missingInputs, "TIMESTAMPED_US10Y")
	}
	if price == nil || price.Close <= 0 {
		missingInputs = append(missingInputs, "KOSPI_CLOSE")
	}

	hasCoreInputs := (price != nil && price.Close > 0) || (volatility != nil && volatility.VKOSPI > 0)
	if !hasCoreInputs {
		s.DomesticMarketStressIdx = nil
		s.DomesticStressStatus = "NOT_EVALUATED"
		s.MissingInputs = missingInputs
	} else {
		domScore := 0.0
		// 1. KOSPI 일간 변동 컴포넌트 (Weight 0.25)
		kospiChg := 0.0
		if price != nil && price.PreviousClose > 0 {
			kospiChg = (price.Close - price.PreviousClose) / price.PreviousClose * 100.0
			if kospiChg < 0 {
				domScore += math.Min(math.Abs(kospiChg)/4.0, 1.0) * 10.0 * 0.25
			}
		}

		// 2. VKOSPI 변동성 컴포넌트 (Weight 0.25)
		vkospiVal := 0.0
		if volatility != nil && volatility.VKOSPI > 0 {
			vkospiVal = volatility.VKOSPI
		}
		if vkospiVal > 0 {
			domScore += math.Min(vkospiVal/40.0, 1.0) * 10.0 * 0.25
		}

		// 3. 외국인 순수급 강도 컴포넌트 (Weight 0.20)
		if impact != nil && impact.ForeignNetFlowToTradingValue != nil {
			domScore += math.Min(math.Abs(*impact.ForeignNetFlowToTradingValue)/25.0, 1.0) * 10.0 * 0.20
		}

		// 4. 안전장치 발동 컴포넌트 (Weight 0.15)
		if impact != nil && impact.SidecarStatus == "triggered" {
			domScore += 1.5
		}

		// 5. 신용 반대매매 컴포넌트 (Weight 0.15)
		forcedSellVal := 0.0
		if credit != nil {
			forcedSellVal = credit.ForcedSellRatioPct
		}
		domScore += math.Min(forcedSellVal/10.0, 1.0) * 10.0 * 0.15

		domResult := math.Round(domScore*10) / 10
		if kospiChg <= -8.0 {
			domResult = math.Max(domResult, 9.0)
		} else if kospiChg <= -7.0 {
			domResult = math.Max(domResult, 8.0)
		} else if kospiChg <= -5.0 {
			domResult = math.Max(domResult, 7.0)
		} else if kospiChg <= -3.0 {
			domResult = math.Max(domResult, 5.0)
		}
		if vkospiVal >= 70.0 {
			domResult = math.Max(domResult, 8.0)
		} else if vkospiVal >= 50.0 {
			domResult = math.Max(domResult, 6.5)
		} else if vkospiVal >= 35.0 {
			domResult = math.Max(domResult, 5.0)
		}
		if impact != nil && impact.SidecarStatus == "triggered" {
			domResult = math.Max(domResult, 6.0)
		}
		s.DomesticMarketStressIdx = &domResult
		if len(missingInputs) == 0 {
			s.DomesticStressStatus = "EVALUATED"
		} else {
			s.DomesticStressStatus = "PARTIALLY_RECONCILED"
			s.MissingInputs = missingInputs
		}
	}

	if firstErr != "" {
		s.Reason = "partial: " + firstErr
	}
	return s
}

// alignUSSeries: KOSPI 거래일 D의 09:00 KST 직전 완결된 가장 최근 NASDAQ 거래일을 매칭
func alignUSSeries(ksSeries, usSeries []yahoo.DailyClose) ([]float64, []float64) {
	kst := time.FixedZone("KST", 9*3600)
	var ksReturns, usReturns []float64

	if len(ksSeries) < 2 || len(usSeries) < 2 {
		return nil, nil
	}

	for i := 1; i < len(ksSeries); i++ {
		ksTime := time.Unix(ksSeries[i].DateUnix, 0).In(kst)
		ksOpenTime := time.Date(ksTime.Year(), ksTime.Month(), ksTime.Day(), 9, 0, 0, 0, kst)

		var bestIdx int = -1
		for j := 1; j < len(usSeries); j++ {
			usTime := time.Unix(usSeries[j].DateUnix, 0)
			if usTime.Before(ksOpenTime) {
				bestIdx = j
			}
		}

		if bestIdx > 0 {
			ksRet := (ksSeries[i].Close - ksSeries[i-1].Close) / ksSeries[i-1].Close * 100.0
			usRet := (usSeries[bestIdx].Close - usSeries[bestIdx-1].Close) / usSeries[bestIdx-1].Close * 100.0
			ksReturns = append(ksReturns, ksRet)
			usReturns = append(usReturns, usRet)
		}
	}
	return ksReturns, usReturns
}

// alignNikkeiSeries: KOSPI 거래일 D의 15:30 KST 직전 완결된 가장 최근 Nikkei 거래일을 매칭
func alignNikkeiSeries(ksSeries, jpSeries []yahoo.DailyClose) ([]float64, []float64) {
	kst := time.FixedZone("KST", 9*3600)
	var ksReturns, jpReturns []float64

	if len(ksSeries) < 2 || len(jpSeries) < 2 {
		return nil, nil
	}

	for i := 1; i < len(ksSeries); i++ {
		ksTime := time.Unix(ksSeries[i].DateUnix, 0).In(kst)
		ksCloseTime := time.Date(ksTime.Year(), ksTime.Month(), ksTime.Day(), 15, 30, 0, 0, kst)

		var bestIdx int = -1
		for j := 1; j < len(jpSeries); j++ {
			jpTime := time.Unix(jpSeries[j].DateUnix, 0)
			if jpTime.Before(ksCloseTime) {
				bestIdx = j
			}
		}

		if bestIdx > 0 {
			ksRet := (ksSeries[i].Close - ksSeries[i-1].Close) / ksSeries[i-1].Close * 100.0
			jpRet := (jpSeries[bestIdx].Close - jpSeries[bestIdx-1].Close) / jpSeries[bestIdx-1].Close * 100.0
			ksReturns = append(ksReturns, ksRet)
			jpReturns = append(jpReturns, jpRet)
		}
	}
	return ksReturns, jpReturns
}

func pearsonCorr(x, y []float64) float64 {
	n := min2(len(x), len(y))
	if n < 3 {
		return 0
	}
	xm, ym := mean(x[:n]), mean(y[:n])
	var num, dX, dY float64
	for i := 0; i < n; i++ {
		dx, dy := x[i]-xm, y[i]-ym
		num += dx * dy
		dX += dx * dx
		dY += dy * dy
	}
	denom := math.Sqrt(dX * dY)
	if denom == 0 {
		return 0
	}
	return num / denom
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func classifyPhase(price *PriceSection, impact *ImpactSection, volatility *VolatilitySection, macro *MacroSection) string {
	var isPanic bool
	var panicLabel string

	if price != nil && price.PreviousClose > 0 {
		chg := (price.Close - price.PreviousClose) / price.PreviousClose * 100.0
		if chg <= -8.0 {
			isPanic = true
			panicLabel = "시장 패닉 (CB 발동 급락)"
		} else if chg <= -5.0 {
			isPanic = true
			panicLabel = "시장 패닉 (급락 국면)"
		} else if chg <= -3.0 {
			isPanic = true
			panicLabel = "급락 국면"
		} else if chg >= 2.0 {
			if volatility != nil && volatility.VKOSPI >= 30.0 {
				return "고변동성 반등"
			}
			return "강세 반등"
		}
	}

	if impact != nil && impact.SidecarStatus == "triggered" {
		isPanic = true
		if panicLabel == "" {
			panicLabel = "시장 패닉 (사이드카 발동)"
		} else {
			panicLabel += " & 사이드카 발동"
		}
	}

	if isPanic {
		return panicLabel
	}

	if volatility != nil && volatility.VKOSPI >= 30.0 {
		return "고변동성 장세"
	}

	if macro == nil {
		return "데이터 없음"
	}
	oilUp := macro.Quotes["CL=F"].ChangePercent > 1.0
	krwUp := macro.Quotes["KRW=X"].ChangePercent > 0.3 // USD/KRW 상승 = 달러 강세
	switch {
	case oilUp && krwUp:
		return "지정학 risk-off"
	case !oilUp && krwUp:
		return "성장 둔화 risk-off"
	case oilUp && !krwUp:
		return "리플레이션"
	case krwUp:
		return "달러 강세"
	default:
		return "특이 신호 없음"
	}
}
