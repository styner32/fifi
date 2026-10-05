package snapshot

import (
	"context"
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
// Index futures daily bars do not establish a completed US cash session.
// Until aligned returns and all model inputs exist, do not publish scores.
func collectRegime(ctx context.Context, yClient YahooQuotes, price *PriceSection, volatility *VolatilitySection, impact *ImpactSection, macro *MacroSection, credit *CreditSection) *RegimeSection {
	s := &RegimeSection{Status: StatusNotEvaluated, Phase: "NOT_EVALUATED", GlobalRiskAversionStatus: "NOT_EVALUATED", DomesticStressStatus: "NOT_EVALUATED", KospiNasdaqCorrTrend: "NOT_EVALUATED", KospiNikkeiCorrTrend: "NOT_EVALUATED", MissingInputs: []string{"ALIGNED_COMPLETED_SESSION_RETURNS", "VALIDATED_COMPLETE_MODEL_INPUTS"}, Reason: "수익률의 거래일·완결 시각 및 모델 입력 검증 미완료"}
	if price != nil {
		s.PriceRegime = price.PriceRegime
	}
	return s
}

// alignUSSeries: KOSPI 거래일 D의 09:00 KST 직전 완결된 가장 최근 NASDAQ 거래일을 매칭
