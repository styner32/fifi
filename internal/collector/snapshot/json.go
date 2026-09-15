package snapshot

import "encoding/json"

// Preserve missingness in machine-readable output as well as Markdown.
func maskedJSON(value any, fields ...string) ([]byte, error) {
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	var obj map[string]json.RawMessage
	if e = json.Unmarshal(raw, &obj); e != nil {
		return nil, e
	}
	for _, f := range fields {
		obj[f] = json.RawMessage("null")
	}
	return json.Marshal(obj)
}
func (p PriceSection) MarshalJSON() ([]byte, error) {
	type plain PriceSection
	return maskedJSON(plain(p), p.MissingFields...)
}
func (c ConcentrationSection) MarshalJSON() ([]byte, error) {
	type plain ConcentrationSection
	var fs []string
	if c.Status != StatusEstimated {
		fs = []string{"top_2_percent", "top_5_percent", "top_10_percent", "hhi", "denominator_eok"}
	}
	return maskedJSON(plain(c), fs...)
}
func (r RegimeSection) MarshalJSON() ([]byte, error) {
	type plain RegimeSection
	return maskedJSON(plain(r), "kospi_nasdaq_corr", "kospi_nikkei_corr", "global_risk_aversion_idx")
}
func (v VolatilitySection) MarshalJSON() ([]byte, error) {
	type plain VolatilitySection
	fs := []string{"decoupling_flag"}
	if v.VKOSPI <= 0 {
		fs = append(fs, "vkospi")
	}
	if !v.VKOSPIChangeOK {
		fs = append(fs, "vkospi_change")
	}
	if len(v.AverageDates) != 5 {
		fs = append(fs, "vkospi_5day_avg")
	}
	if v.VIX <= 0 {
		fs = append(fs, "vix")
	}
	if !v.VIXChangeOK {
		fs = append(fs, "vix_change")
	}
	if v.ObservedAt.IsZero() {
		fs = append(fs, "observed_at")
	}
	if v.VIXObservedAt.IsZero() {
		fs = append(fs, "vix_observed_at")
	}
	return maskedJSON(plain(v), fs...)
}
func (q QuoteSummary) MarshalJSON() ([]byte, error) {
	type plain QuoteSummary
	fs := []string{}
	if !finite(q.PreviousClose) || q.PreviousClose <= 0 {
		fs = append(fs, "previous_close", "change_percent")
	} else {
		q.ChangePercent = (q.Price/q.PreviousClose - 1) * 100
	}
	if q.MarketTimeUnix <= 0 {
		fs = append(fs, "market_time_unix")
	}
	return maskedJSON(plain(q), fs...)
}
func (c CreditSection) MarshalJSON() ([]byte, error) {
	type plain CreditSection
	fs := append([]string(nil), c.MissingFields...)
	fs = append(fs, "margin_receivable_prev_eok", "margin_receivable_delta_eok", "forced_sell_amount_prev_eok", "forced_sell_delta_eok")
	if c.KofiaDate == "" {
		fs = append(fs, "margin_receivable_eok", "forced_sell_amount_eok", "forced_sell_ratio_pct", "raw_forced_sell_mln", "raw_margin_receivable_mln")
	}
	return maskedJSON(plain(c), fs...)
}
func (s LateSessionSection) MarshalJSON() ([]byte, error) {
	type plain LateSessionSection
	fs := []string{"is_expiring_contract", "kospi200_standard_futures_expiry", "kospi200_monthly_options_expiry"}
	if !s.PatternEvaluated {
		fs = append(fs, "pattern_detected")
	}
	if !s.BasisOK {
		fs = append(fs, "basis_point", "basis_rate", "futures_price", "spot_price")
	}
	if !s.Futures1530OK {
		fs = append(fs, "futures_price_1530", "basis_point_1530")
	}
	if !s.ProgramOK {
		fs = append(fs, "kospi_net_arbitrage_total", "kospi_net_non_arbitrage_total", "kospi_program_total_net", "original_snapshot_arbitrage_eok", "original_snapshot_non_arbitrage_eok", "original_snapshot_total_eok")
	}
	if !s.InstitutionOK {
		fs = append(fs, "kospi_net_non_arbitrage_organ")
	}
	if !s.ForeignOK {
		fs = append(fs, "kospi_net_non_arbitrage_foreign")
	}
	if s.NaverFollowupTotalEok == nil {
		fs = append(fs, "cross_source_difference_eok")
	}
	if s.FuturesObservedAt.IsZero() {
		fs = append(fs, "futures_observed_at")
	}
	if s.SpotObservedAt.IsZero() {
		fs = append(fs, "spot_observed_at")
	}
	return maskedJSON(plain(s), fs...)
}

func (s ImpactSection) MarshalJSON() ([]byte, error) {
	type plain ImpactSection
	fs := []string{}
	if s.FuturesObservedAt.IsZero() {
		fs = append(fs, "futures_observed_at")
	}
	if s.TotalKospiMarketCapEok <= 0 {
		fs = append(fs, "total_kospi_market_cap_eok")
	}
	return maskedJSON(plain(s), fs...)
}
