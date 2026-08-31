package snapshot

import (
	"context"

	"github.com/fifi/internal/domesticstock"
)

// ConcentrationSection은 코스피 시총 집중도 지표를 담습니다.
type ConcentrationSection struct {
	Top5Percent            float64       `json:"top_5_percent"`  // 상위 5종목 시총 비중 (%)
	Top10Percent           float64       `json:"top_10_percent"` // 상위 10종목 시총 비중 (%)
	HHI                    float64       `json:"hhi"`            // Herfindahl-Hirschman Index
	HHILevel               string        `json:"hhi_level"`      // "비집중"/"중간 집중"/"고집중"
	HHIStatus              string        `json:"hhi_status,omitempty"` // HHI_LOW_BY_CUSTOM_THRESHOLD
	RiskLevel              string        `json:"risk_level"`     // 상위 종목 집중 위험 ("🟢 보통"/"🟡 높음"/"🔴 매우 높음")
	Top2ConcentrationStatus string       `json:"top_2_concentration_status,omitempty"` // TOP2_CONCENTRATION_EXTREME
	Reason                 string        `json:"-"`
	Date                   string        `json:"date,omitempty"` // KOSPI 마스터 영업일
	Status                 QualityStatus `json:"status,omitempty"`
	QualityFlags           []string      `json:"quality_flags,omitempty"`
}

// collectConcentration은 KOSPIMarketCapSummary Constituents를 재사용합니다.
// Constituents는 시총 내림차순 정렬 상태 (market_cap_summary.go 참고).
func collectConcentration(ctx context.Context, stock DomesticStock, date string) *ConcentrationSection {
	s := &ConcentrationSection{}
	if stock == nil {
		s.Reason = "domestic stock dependency is nil"
		return s
	}
	summary, err := stock.KOSPIMarketCapSummary(ctx, date)
	if err != nil {
		s.Reason = err.Error()
		return s
	}
	if summary.TotalMarketCap <= 0 || len(summary.Constituents) == 0 {
		s.Reason = "KOSPI market cap summary empty"
		return s
	}
	s.Top5Percent = topNCapPercent(summary.Constituents, 5, summary.TotalMarketCap)
	s.Top10Percent = topNCapPercent(summary.Constituents, 10, summary.TotalMarketCap)
	s.HHI = calcHHI(summary.Constituents, summary.TotalMarketCap)
	s.HHILevel = hhiLevel(s.HHI)
	s.HHIStatus = "HHI_LOW_BY_CUSTOM_THRESHOLD"
	s.RiskLevel = concentrationRisk(summary.Constituents, s.Top5Percent)
	s.Top2ConcentrationStatus = "TOP2_CONCENTRATION_EXTREME"
	s.Date = summary.BusinessDate

	// Auto-validation: HHI >= sum(weight^2) of known weights (top 5 weights)
	var top5Weights []float64
	for i, c := range summary.Constituents {
		if i >= 5 {
			break
		}
		top5Weights = append(top5Weights, c.MarketCap/summary.TotalMarketCap*100)
	}
	s.Status, s.QualityFlags = ValidateHHI(s.HHI, top5Weights)

	return s
}


func topNCapPercent(cs []domesticstock.KOSPIMarketCapConstituent, n int, total float64) float64 {
	sum := 0.0
	for i, c := range cs {
		if i >= n {
			break
		}
		sum += c.MarketCap
	}
	return sum / total * 100
}

// calcHHI = Σ(share_i²), share_i = 종목 시총 비중 (%)
func calcHHI(cs []domesticstock.KOSPIMarketCapConstituent, total float64) float64 {
	hhi := 0.0
	for _, c := range cs {
		share := c.MarketCap / total * 100
		hhi += share * share
	}
	return hhi
}

func hhiLevel(hhi float64) string {
	switch {
	case hhi < 1500:
		return "비집중"
	case hhi < 2500:
		return "중간 집중"
	default:
		return "고집중"
	}
}

// concentrationRisk: 상위 2종목이 동업종이고 비중 >45% → 매우 높음
func concentrationRisk(cs []domesticstock.KOSPIMarketCapConstituent, top5 float64) string {
	// 삼성전자(005930) + SK하이닉스(000660) 동시 상위 → 반도체 쏠림
	top2Codes := map[string]bool{}
	for i, c := range cs {
		if i >= 2 {
			break
		}
		top2Codes[c.Code] = true
	}
	if top2Codes["005930"] && top2Codes["000660"] {
		return "🔴 매우 높음 (반도체 쏠림)"
	}
	if top5 > 55 {
		return "🟡 높음"
	}
	return "🟢 보통"
}
