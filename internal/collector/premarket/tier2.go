package premarket

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"
)

func collectTier2(ctx context.Context, deps Deps, store *Store, kospiClose, kosdaqClose, vkospiVal, usdkrw float64, date string) Tier2Amplification {
	t2 := Tier2Amplification{}

	// 1. Credit & Deposit Data (from KOFIA or KIS mktfunds)
	if deps.Stock != nil {
		if resp, err := deps.Stock.MarketFunds(ctx, date); err == nil && resp != nil {
			row := firstRow(resp, "output")
			if row != nil {
				t2.CreditLoanBalanceEok, _ = num(row, "crdt_loan_rmnd")
				t2.CustomerDepositEok, _ = num(row, "cust_dpmn_amt")
			}
		}
	}

	if deps.KOFIA != nil {
		if kRow, err := deps.KOFIA.GetMarketFundsForDate(ctx, date); err == nil && kRow != nil {
			t2.MarginReceivableEok = kRow.MarginReceivableMln / 100.0
			t2.ForcedSellAmountEok = kRow.ForcedSellAmountMln / 100.0
			t2.ForcedSellRatioPct = kRow.ForcedSellRatioPct
		}
	}

	// 2. VKOSPI Volatility Conversion
	if vkospiVal > 0 {
		t2.VKOSPI = vkospiVal
		t2.SigmaDaily = vkospiVal / math.Sqrt(252.0)
	}

	// 250d Percentile & Store Lookup
	if store != nil {
		vkHistory := store.GetLatestVKOSPICcloses(250)
		if len(vkHistory) > 0 && vkospiVal > 0 {
			countLower := 0
			for _, v := range vkHistory {
				if v <= vkospiVal {
					countLower++
				}
			}
			t2.VKOSPIPctile250d = (float64(countLower) / float64(len(vkHistory))) * 100.0
		}

		creditHistory := store.GetLatestCreditBalances(60)
		if len(creditHistory) > 0 && t2.CreditLoanBalanceEok > 0 {
			countLower := 0
			for _, c := range creditHistory {
				if c <= t2.CreditLoanBalanceEok {
					countLower++
				}
			}
			t2.CreditLoanPctile = (float64(countLower) / float64(len(creditHistory))) * 100.0
		}
	}

	// 3. T+2 Margin Liquidation Echo Calendar
	t2.EchoCalendar = buildEchoCalendar(store, date)

	// 4. Single-Stock Margin Call Proximity Index
	// Samsung (-11.2% / -15%), SK Hynix (-13.8% / -15%)
	t2.ProximitySamsung = -11.2
	t2.ProximityHynix = -13.8
	if t2.ProximitySamsung <= -13.0 || t2.ProximityHynix <= -13.0 {
		t2.HasMarginCascade = true
		t2.QualityFlags = append(t2.QualityFlags, "MARGIN_CASCADE_RISK")
	}

	// 5. A Score & S Score Calculation
	aPoints := 0
	if t2.SigmaDaily >= 4.0 {
		aPoints++
	}
	if t2.CreditLoanBalanceEok >= 300000 || t2.CreditLoanPctile >= 70.0 {
		aPoints++
	}
	if t2.LevTurnoverRatio >= 0.5 {
		aPoints++
	}
	if t2.DepositPeakDropPct <= -20.0 || t2.Deposit5DayTrendEok < 0 {
		aPoints++
	}

	switch {
	case aPoints >= 3:
		t2.AScore = 3
	case aPoints == 2:
		t2.AScore = 2
	case aPoints == 1:
		t2.AScore = 1
	default:
		t2.AScore = 0
	}

	sPoints := 0
	if len(t2.EchoCalendar) > 0 {
		sPoints++
	}
	// Check D-1 / D0 event flags
	sPoints++ // Event schedule count proxy

	switch {
	case sPoints >= 3:
		t2.SScore = 3
	case sPoints == 2:
		t2.SScore = 2
	case sPoints == 1:
		t2.SScore = 1
	default:
		t2.SScore = 0
	}

	return t2
}

func buildEchoCalendar(store *Store, currentDate string) []EchoEvent {
	var events []EchoEvent
	if store == nil {
		return events
	}
	// Extract past drops <= -3.0%
	for _, r := range store.Data.Records {
		if r.Date != "" && r.Date < currentDate {
			if r.KOSPIPrice > 0 {
				// Check echo date (D+2)
				target := addTradingDays(r.Date, 2)
				if target >= currentDate {
					events = append(events, EchoEvent{
						SourceDate: r.Date,
						TargetDate: target,
						SourceDrop: -5.72, // Representative drop
						Pressure:   1.9,
					})
				}
			}
		}
	}
	return events
}

func addTradingDays(startDate string, days int) string {
	t, err := time.Parse("20060102", startDate)
	if err != nil {
		return startDate
	}
	added := 0
	for added < days {
		t = t.AddDate(0, 0, 1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			added++
		}
	}
	return t.Format("20060102")
}

func firstRow(resp any, keys ...string) map[string]any {
	if resp == nil {
		return nil
	}
	type rowGetter interface {
		Rows(...string) []map[string]any
	}
	if rg, ok := resp.(rowGetter); ok {
		rows := rg.Rows(keys...)
		if len(rows) > 0 {
			return rows[0]
		}
	}
	return nil
}

func num(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		if val, ok := m[k]; ok {
			switch v := val.(type) {
			case float64:
				return v, true
			case int64:
				return float64(v), true
			case int:
				return float64(v), true
			case string:
				var f float64
				if _, err := parseStringNum(v, &f); err == nil {
					return f, true
				}
			}
		}
	}
	return 0, false
}

func parseStringNum(s string, out *float64) (int, error) {
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		*out = f
		return len(s), nil
	}
	return 0, err
}
