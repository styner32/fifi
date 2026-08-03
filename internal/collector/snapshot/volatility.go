package snapshot

import (
	"context"
	"fmt"
	"time"
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
	Reason         string        `json:"-"`
	Source         string        `json:"source"`
	Status         QualityStatus `json:"status,omitempty"`
	QualityFlags   []string      `json:"quality_flags,omitempty"`
	ObservedAt     time.Time     `json:"observed_at,omitempty"`
}

// collectVolatility는 VKOSPI(Naver Finance)와 VIX(Yahoo Finance)를 조회합니다.
func collectVolatility(ctx context.Context, stock DomesticStock, naverClient NaverFinance, yahoo YahooQuotes, indexChange float64, date string, opts Options) *VolatilitySection {
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

	// KIS 지수 API를 우선 사용한다. Naver는 KIS가 값을 주지 못할 때만 fallback이다.
	if stock != nil {
		if code, resolveErr := stock.ResolveVKOSPICode(ctx, []string{"0503", "2050"}); resolveErr == nil {
			if resp, priceErr := stock.InquireVKOSPIPrice(ctx, code); priceErr == nil {
				if row := firstRow(resp, "output"); row != nil {
					vk, vkOK := num(row, "bstp_nmix_prpr")
					change, _ := num(row, "bstp_nmix_prdy_ctrt")
					if vkOK && vk >= 5 && vk <= 100 {
						s.VKOSPI = vk
						s.VKOSPIChange = change
						s.Level = vkospiLevel(vk)
						s.DecouplingFlag = isDecoupling(indexChange, change)
						s.Source = "KIS"
						fromDate := time.Now().AddDate(0, 0, -14).Format("20060102")
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
						} else {
							s.Reason = appendReason(s.Reason, "KIS VKOSPI 5d avg: "+histErr.Error())
						}
						return s
					}
				}
			}
		}
	}

	if naverClient != nil {
		// ── VKOSPI 현재가 ────────────────────────────────────────────────────────
		// 1차: polling API (장중 실시간)
		// 2차: sise_index_day 최근 종가 (장 마감 후 fallback)
		quote, err := naverClient.GetIndexQuote(ctx, "VKOSPI")
		if err != nil {
			// polling 실패 → sise_index_day에서 최근 종가로 fallback
			history, histErr := naverClient.GetIndexDailyHistory(ctx, "VKOSPI", 10)
			if histErr == nil && len(history) > 0 {
				// 가장 최근 종가 사용
				last := history[len(history)-1]
				vk := last.Close
				if vk >= 5 && vk <= 100 {
					s.VKOSPI = vk
					s.Level = vkospiLevel(vk)
					s.Source = "Naver"
					// 5일 평균
					if len(history) >= 5 {
						sum, count := 0.0, 0
						for i := len(history) - 1; i >= 0 && count < 5; i-- {
							sum += history[i].Close
							count++
						}
						s.VKOSPI5DayAvg = sum / float64(count)
					}
					return s
				} else {
					s.Reason = appendReason(s.Reason, fmt.Sprintf("VKOSPI 종가 %.2f 정상범위 초과", vk))
				}
			} else if histErr != nil {
				s.Reason = appendReason(s.Reason, fmt.Sprintf("VKOSPI unavailable (polling: %v, history: %v)", err, histErr))
			}
		} else if quote != nil {
			vk := quote.Price
			if vk >= 5 && vk <= 100 {
				s.VKOSPI = vk
				s.VKOSPIChange = quote.ChangePercent
				s.Level = vkospiLevel(vk)
				s.DecouplingFlag = isDecoupling(indexChange, quote.ChangePercent)
				s.Source = "Naver"

				// ── VKOSPI 5거래일 평균 ───────────────────────────────────────────────────
				history, histErr := naverClient.GetIndexDailyHistory(ctx, "VKOSPI", 10)
				if histErr == nil && len(history) >= 5 {
					sum, count := 0.0, 0
					for i := len(history) - 1; i >= 0 && count < 5; i-- {
						sum += history[i].Close
						count++
					}
					s.VKOSPI5DayAvg = sum / float64(count)
				} else if histErr != nil {
					s.Reason = appendReason(s.Reason, "VKOSPI 5d avg: "+histErr.Error())
				}
				return s
			} else {
				s.Reason = appendReason(s.Reason, fmt.Sprintf("VKOSPI %.2f 정상범위(5~100) 초과 — 데이터 이상", vk))
			}
		}
	}

	// ── fallback: read today's snapshot file if it exists and has valid VKOSPI ──
	if s.VKOSPI == 0 {
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
			s.Status = StatusStale
			s.Reason = appendReason(s.Reason, "VKOSPI collection failed; fallback to last successful snapshot")
			return s
		}
	}

	if s.VKOSPI == 0 {
		s.Status = StatusUnavailable
		s.QualityFlags = []string{"VKOSPI_UNAVAILABLE"}
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
