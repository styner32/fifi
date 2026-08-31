package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/fifi/internal/market/facts"
)

// VolatilitySection은 VKOSPI/VIX 변동성 지표를 담습니다.
type VolatilitySection struct {
	VKOSPI         float64       `json:"vkospi"`
	VKOSPIChange   float64       `json:"vkospi_change"`
	VKOSPI5DayAvg  float64       `json:"vkospi_5day_avg"`
	VIX            float64       `json:"vix"`
	VIXChange      float64       `json:"vix_change"`
	DecouplingFlag bool          `json:"decoupling_flag"`
	Level          string        `json:"level"`
	Reason         string        `json:"reason,omitempty"`
	Source         string        `json:"source"`
	Status         QualityStatus `json:"status,omitempty"`
	QualityFlags   []string      `json:"quality_flags,omitempty"`
	ObservedAt     time.Time     `json:"observed_at,omitempty"`
}

// collectVolatility는 VKOSPI(KIS -> Naver -> facts.Store fallback)와 VIX(Yahoo Finance)를 조회합니다.
func collectVolatility(ctx context.Context, stock DomesticStock, naverClient NaverFinance, yahoo YahooQuotes, factsStore facts.Store, indexChange float64, date string, opts Options) *VolatilitySection {
	s := &VolatilitySection{Status: StatusValid, ObservedAt: time.Now()}

	// VIX는 VKOSPI 성공 여부와 무관하게 항상 조회
	if yahoo != nil {
		if quotes, yErr := yahoo.GetQuotes(ctx, []string{"^VIX"}); yErr == nil {
			if q, ok := quotes["^VIX"]; ok {
				s.VIX = q.Price
				s.VIXChange = q.ChangePercent
			}
		} else {
			s.Reason = appendReason(s.Reason, "VIX: "+yErr.Error())
		}
	}

	// facts.Resolve를 사용하여 VKOSPI 조회 및 store 저장 / fallback 수행
	obs := facts.Resolve(ctx, factsStore, "vkospi.value", 7*24*time.Hour, func(fetchCtx context.Context) (facts.Observation, error) {
		// 1. KIS 우선 조회
		if stock != nil {
			code, resolveErr := stock.ResolveVKOSPICode(fetchCtx, nil)
			if resolveErr == nil {
				resp, priceErr := stock.InquireVKOSPIPrice(fetchCtx, code)
				if priceErr == nil && resp.IsOK() {
					row := firstRow(resp, "output")
					if row != nil {
						if vk, vkOK := num(row, "bstp_nmix_prpr"); vkOK && vk >= 5 && vk <= 100 {
							change, _ := num(row, "bstp_nmix_prdy_ctrt")
							s.VKOSPIChange = change
							return facts.Observation{
								MetricID:        "vkospi.value",
								BusinessDate:    date,
								ObservedAt:      time.Now(),
								Value:           &vk,
								Source:          "KIS",
								SourceField:     "bstp_nmix_prpr",
								FreshnessStatus: string(StatusValid),
								Raw:             row,
							}, nil
						}
					}
				}
			}
		}

		// 2. Naver fallback
		if naverClient != nil {
			quote, err := naverClient.GetIndexQuote(fetchCtx, "VKOSPI")
			if err == nil && quote != nil && quote.Price >= 5 && quote.Price <= 100 {
				s.VKOSPIChange = quote.ChangePercent
				return facts.Observation{
					MetricID:        "vkospi.value",
					BusinessDate:    date,
					ObservedAt:      time.Now(),
					Value:           &quote.Price,
					Source:          "Naver",
					FreshnessStatus: string(StatusValid),
				}, nil
			}
			history, histErr := naverClient.GetIndexDailyHistory(fetchCtx, "VKOSPI", 10)
			if histErr == nil && len(history) > 0 {
				last := history[len(history)-1]
				if last.Close >= 5 && last.Close <= 100 {
					return facts.Observation{
						MetricID:        "vkospi.value",
						BusinessDate:    date,
						ObservedAt:      time.Now(),
						Value:           &last.Close,
						Source:          "NaverHistory",
						FreshnessStatus: string(StatusValid),
					}, nil
				}
			}
		}

		return facts.Observation{}, fmt.Errorf("all VKOSPI live sources failed")
	})

	if obs.Value != nil && *obs.Value > 0 {
		s.VKOSPI = *obs.Value
		s.Level = vkospiLevel(s.VKOSPI)
		s.DecouplingFlag = isDecoupling(indexChange, s.VKOSPIChange)
		s.Source = obs.Source
		s.Status = QualityStatus(obs.FreshnessStatus)
		if obs.FreshnessStatus == "PROVISIONAL_LAST_VALID" {
			s.Reason = appendReason(s.Reason, obs.MissingReason)
			s.ObservedAt = obs.ObservedAt
		}

		// 5일 평균 조회
		if stock != nil {
			fromDate := time.Now().AddDate(0, 0, -14).Format("20060102")
			if code, err := stock.ResolveVKOSPICode(ctx, nil); err == nil {
				if history, histErr := stock.InquireVKOSPIDailyPrice(ctx, code, fromDate); histErr == nil {
					sum, count := 0.0, 0
					for _, historyRow := range history {
						if closeValue, ok := num(historyRow, "bstp_nmix_prpr", "stck_clpr"); ok && closeValue >= 5 && closeValue <= 100 {
							sum += closeValue
							count++
							if count == 5 {
								break
							}
						}
					}
					if count > 0 {
						s.VKOSPI5DayAvg = sum / float64(count)
					}
				}
			}
		}

		if s.VIX > 0 {
			ratio := s.VKOSPI / s.VIX
			if ratio > 3.5 || ratio < 0.28 {
				s.QualityFlags = append(s.QualityFlags, "INCONSISTENT_WITH_RELATED_METRIC")
			}
		}
		return s
	}

	// ── fallback to local snapshot file if facts.Store was empty ──
	outDir := opts.OutputDir
	if outDir == "" {
		outDir = ".cache/snapshots"
	}
	if todaySnap, err := LoadSnapshotForDate(outDir, date); err == nil && todaySnap != nil && todaySnap.Volatility != nil && todaySnap.Volatility.VKOSPI > 0 {
		s.VKOSPI = todaySnap.Volatility.VKOSPI
		s.VKOSPIChange = todaySnap.Volatility.VKOSPIChange
		s.VKOSPI5DayAvg = todaySnap.Volatility.VKOSPI5DayAvg
		s.Level = vkospiLevel(s.VKOSPI)
		s.DecouplingFlag = isDecoupling(indexChange, s.VKOSPIChange)
		s.Source = todaySnap.Volatility.Source
		s.Status = StatusProvisionalLastValid
		s.Reason = appendReason(s.Reason, "VKOSPI collection failed; fallback to last successful snapshot")
		return s
	}

	s.Status = StatusUnavailable
	s.QualityFlags = []string{"VKOSPI_UNAVAILABLE"}
	if obs.MissingReason != "" {
		s.Reason = appendReason(s.Reason, obs.MissingReason)
	}

	return s
}


func vkospiLevel(v float64) string {
	switch {
	case v < 20:
		return "정상"
	case v < 25:
		return "평상시"
	case v < 30:
		return "주의"
	default:
		return "위험"
	}
}

// isDecoupling: 지수와 VKOSPI 방향이 다르고 VKOSPI 변동폭 > 5%
func isDecoupling(indexChg, vkospiChg float64) bool {
	if vkospiChg < -5 || vkospiChg > 5 {
		return (indexChg > 0 && vkospiChg < -5) || (indexChg < 0 && vkospiChg > 5)
	}
	return false
}
