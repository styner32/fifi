package premarket

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/parse"
	"golang.org/x/sync/errgroup"
)

var cashSymbols = []string{"^GSPC", "^IXIC", "^DJI", "^VIX", "^TNX", "EWY", "SKHY", "^SOX", "MU", "NVDA", "ASML"}
var quoteSymbols = []string{"CL=F", "BZ=F", "DX-Y.NYB", "NQ=F", "KRW=X"}

func missing(reason string, now time.Time) Observation {
	return Observation{Status: "MISSING", Reason: reason, Session: "UNKNOWN", FetchedAt: now}
}
func emptyInputs(now time.Time) map[string]Observation {
	in := map[string]Observation{}
	for _, keys := range [][]string{cashSymbols, quoteSymbols, {"kospi", "kosdaq", "vkospi", "foreign_flow", "credit", "deposit", "margin", "forced", "forced_ratio", "semi_composite"}} {
		for _, key := range keys {
			in[key] = missing("DATA_UNAVAILABLE", now)
		}
	}
	for key, reason := range map[string]string{
		"adr_premium":      "ADR_RATIO_AND_ALIGNED_PRICES_UNVERIFIED",
		"ndf_gap":          "NDF_SOURCE_NOT_CONFIGURED",
		"divergence":       "FUTURES_CONTRACT_AND_REFERENCE_UNVERIFIED",
		"lev_turnover":     "NOT_IMPLEMENTED",
		"deposit_trend":    "NOT_IMPLEMENTED",
		"margin_proximity": "COLLATERAL_INPUTS_NOT_AVAILABLE",
		"events":           "EVENT_SOURCE_NOT_CONFIGURED",
		"echo":             "VERIFIED_DROP_AND_SETTLEMENT_CALENDAR_NOT_AVAILABLE",
	} {
		in[key] = missing(reason, now)
	}
	return in
}

type inputJob struct {
	key string
	run func() (Observation, error)
}
type inputResult struct {
	key   string
	value Observation
	err   error
}

func collectInputs(ctx context.Context, deps Deps, date string, cutoff, now time.Time, errs map[string]string) (map[string]Observation, string) {
	in := emptyInputs(now)
	prior := ""
	if deps.Stock != nil {
		resp, err := deps.Stock.MarketTime(ctx)
		if err != nil {
			errs["market_time"] = err.Error()
		} else if row := firstRow(resp, "output1"); row != nil {
			for _, key := range []string{"date1", "date2", "date3", "date4", "date5"} {
				d := normalizeDate(parse.String(row[key]))
				if d != "" && d < date && d > prior {
					prior = d
				}
			}
		}
	}
	jobs := []inputJob{}
	for _, symbol := range cashSymbols {
		symbol := symbol
		jobs = append(jobs, inputJob{symbol, func() (Observation, error) {
			if deps.Yahoo == nil {
				return missing("YAHOO_NOT_CONFIGURED", now), nil
			}
			h, err := deps.Yahoo.GetChartHistory(ctx, symbol, "1mo", "1d")
			if err != nil {
				return missing("DAILY_HISTORY_FAILED", now), err
			}
			return completedUSClose(h, symbol, cutoff, now), nil
		}})
	}
	for _, item := range []struct{ key, code, symbol string }{{"kospi", "0001", "^KS11"}, {"kosdaq", "1001", "^KQ11"}, {"vkospi", "", ""}} {
		item := item
		jobs = append(jobs, inputJob{item.key, func() (Observation, error) {
			return domesticClose(ctx, deps, item.key, item.code, item.symbol, date, prior, now)
		}})
	}
	ch := make(chan inputResult, len(jobs))
	var g errgroup.Group
	g.SetLimit(4)
	for _, j := range jobs {
		j := j
		g.Go(func() error { o, e := j.run(); ch <- inputResult{j.key, o, e}; return nil })
	}
	_ = g.Wait()
	close(ch)
	for result := range ch {
		in[result.key] = result.value
		if result.err != nil {
			errs[result.key] = result.err.Error()
		}
	}
	// US cash components must refer to the same latest completed benchmark date.
	if anchor := in["^GSPC"]; anchor.available() {
		for _, symbol := range cashSymbols {
			o := in[symbol]
			if o.available() && o.BusinessDate != anchor.BusinessDate {
				in[symbol] = missing("US_SESSION_DATE_MISMATCH", now)
			}
		}
	}
	if prior == "" {
		prior = in["kospi"].BusinessDate
	}
	if deps.Yahoo != nil {
		quotes, err := deps.Yahoo.GetQuotes(ctx, quoteSymbols)
		if err != nil {
			errs["yahoo_quotes"] = err.Error()
		}
		for _, symbol := range quoteSymbols {
			in[symbol] = providerQuote(quotes[symbol], symbol, cutoff, now)
			// When re-run after the opening, today's quote cannot reconstruct
			// the premarket observation. Retain a completed provider daily bar
			// as clearly labelled context, never as an exchange settlement.
			if !in[symbol].available() {
				h, histErr := deps.Yahoo.GetChartHistory(ctx, symbol, "1mo", "1d")
				if histErr != nil {
					errs["daily_"+symbol] = histErr.Error()
				} else if o := completedProviderBar(h, symbol, cutoff, now); o.available() {
					in[symbol] = o
				}
			}
		}
	}
	if deps.Stock != nil {
		if prior != "" {
			resp, err := deps.Stock.InquireInvestorDailyByMarket(ctx, prior)
			if err != nil {
				errs["foreign_flow"] = err.Error()
			} else if responseOK(resp) {
				row := datedRow(resp.Rows("output"), "stck_bsop_date", date, prior)
				in["foreign_flow"] = rowNumber(row, "frgn_ntby_tr_pbmn", "stck_bsop_date", "KIS investor daily", "억원", 100, now)
				o := in["foreign_flow"]
				o.Session = "DAILY_FLOW"
				in["foreign_flow"] = o
			}
		}
		resp, err := deps.Stock.MarketFunds(ctx, date)
		if err != nil {
			errs["market_funds"] = err.Error()
		} else if responseOK(resp) {
			row := datedRow(resp.Rows("output"), "bsop_date", date, "")
			for key, field := range map[string]string{"credit": "crdt_loan_rmnd", "deposit": "cust_dpmn_amt"} {
				o := rowNumber(row, field, "bsop_date", "KIS mktfunds", "억원", 1, now)
				if o.available() {
					o.Status = "LAGGED_CONTEXT_ONLY"
					o.Session = "DAILY_STATISTIC"
				}
				in[key] = o
			}
		}
	}
	if deps.KOFIA != nil {
		row, err := deps.KOFIA.GetMarketFundsForDate(ctx, date)
		if err != nil {
			errs["kofia"] = err.Error()
		} else if row != nil {
			d := normalizeDate(row.Date)
			if d != "" && d < date && (row.MarginReceivableMln > 0 || row.ForcedSellAmountMln > 0) {
				for key, val := range map[string]float64{"margin": row.MarginReceivableMln / 100, "forced": row.ForcedSellAmountMln / 100, "forced_ratio": row.ForcedSellRatioPct} {
					if !finite(val) || val < 0 {
						continue
					}
					unit := "억원"
					if key == "forced_ratio" {
						unit = "%"
					}
					in[key] = Observation{Value: ptr(val), BusinessDate: d, Source: "KOFIA FreeSIS", Unit: unit, Session: "DAILY_STATISTIC", Status: "LAGGED_CONTEXT_ONLY", FetchedAt: now}
				}
			} else {
				for _, key := range []string{"margin", "forced", "forced_ratio"} {
					in[key] = missing("SOURCE_DATE_OR_ZERO_UNCONFIRMED", now)
				}
			}
		}
	}
	return in, prior
}

func completedUSClose(h []yahoo.DailyClose, symbol string, cutoff, now time.Time) Observation {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return missing("EXCHANGE_TIMEZONE_UNAVAILABLE", now)
	}
	// Daily bars can contain the currently forming session. The exchange date,
	// rather than the KST arrival date, determines eligibility.
	byDate := map[string]yahoo.DailyClose{}
	for _, bar := range h {
		ts := time.Unix(bar.DateUnix, 0).In(loc)
		closeAt := time.Date(ts.Year(), ts.Month(), ts.Day(), 16, 0, 0, 0, loc)
		if !finite(bar.Close) || bar.Close <= 0 || closeAt.After(cutoff) {
			continue
		}
		byDate[ts.Format("20060102")] = bar
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	if len(dates) == 0 {
		return missing("NO_COMPLETED_DAILY_BAR", now)
	}
	d := dates[len(dates)-1]
	bar := byDate[d]
	asof := time.Unix(bar.DateUnix, 0)
	if cutoff.Sub(asof) > 7*24*time.Hour {
		return missing("STALE_DAILY_BAR", now)
	}
	unit := "points"
	if symbol == "^TNX" {
		unit = "%"
	} else if !strings.HasPrefix(symbol, "^") {
		unit = "USD"
	}
	o := Observation{Value: ptr(bar.Close), Source: "Yahoo " + symbol + " daily", Unit: unit, BusinessDate: d, AsOf: &asof, Session: "DAILY_CLOSE", Status: "VALID", FetchedAt: now}
	if len(dates) > 1 {
		prevDate := dates[len(dates)-2]
		prev := byDate[prevDate].Close
		o.PreviousClose = ptr(prev)
		o.ReferenceDate = prevDate
		o.ChangePct = ptr((bar.Close/prev - 1) * 100)
	}
	return o
}

func providerQuote(q yahoo.Quote, symbol string, cutoff, now time.Time) Observation {
	o := missing("QUOTE_OR_REFERENCE_TIME_UNAVAILABLE", now)
	if !finite(q.Price) || q.Price <= 0 || q.MarketTimeUnix <= 0 {
		return o
	}
	ts := time.Unix(q.MarketTimeUnix, 0)
	if ts.After(cutoff) {
		return missing("AFTER_PREMARKET_CUTOFF", now)
	}
	if cutoff.Sub(ts) > 7*24*time.Hour {
		return missing("STALE_QUOTE", now)
	}
	unit := "USD"
	if symbol == "KRW=X" {
		unit = "KRW/USD"
	} else if symbol == "NQ=F" || symbol == "DX-Y.NYB" {
		unit = "points"
	}
	o = Observation{Value: ptr(q.Price), Source: "Yahoo " + symbol, Unit: unit, AsOf: &ts, Session: "PROVIDER_QUOTE", Status: "VALID", FetchedAt: now, Reason: "provider quote; close/settlement and futures contract not verified"}
	if finite(q.PreviousClose) && q.PreviousClose > 0 {
		o.PreviousClose = ptr(q.PreviousClose)
		o.ChangePct = ptr((q.Price/q.PreviousClose - 1) * 100)
	}
	return o
}

func completedProviderBar(history []yahoo.DailyClose, symbol string, cutoff, now time.Time) Observation {
	// The shared Yahoo adapter exposes timestamps, not each product's trading
	// calendar. Require a full 24h after the bar start and disclose that timestamp.
	// Do not assign a guessed exchange business date or a settlement label.
	h := append([]yahoo.DailyClose(nil), history...)
	sort.Slice(h, func(i, j int) bool { return h[i].DateUnix < h[j].DateUnix })
	var eligible []yahoo.DailyClose
	for _, bar := range h {
		if finite(bar.Close) && bar.Close > 0 && !time.Unix(bar.DateUnix, 0).Add(24*time.Hour).After(cutoff) {
			eligible = append(eligible, bar)
		}
	}
	if len(eligible) == 0 {
		return missing("NO_COMPLETED_PROVIDER_BAR", now)
	}
	bar := eligible[len(eligible)-1]
	asof := time.Unix(bar.DateUnix, 0)
	if cutoff.Sub(asof) > 7*24*time.Hour {
		return missing("STALE_DAILY_BAR", now)
	}
	o := providerQuote(yahoo.Quote{Price: bar.Close, MarketTimeUnix: bar.DateUnix}, symbol, cutoff, now)
	o.Session = "PROVIDER_DAILY"
	o.Source = "Yahoo " + symbol + " daily"
	o.Reason = "provider daily bar; exchange trading date, settlement and contract unverified"
	if len(eligible) > 1 {
		previous := eligible[len(eligible)-2].Close
		referenceTime := time.Unix(eligible[len(eligible)-2].DateUnix, 0)
		o.ReferenceAsOf = &referenceTime
		o.PreviousClose = ptr(previous)
		o.ChangePct = ptr((bar.Close/previous - 1) * 100)
	}
	return o
}

func domesticClose(ctx context.Context, deps Deps, key, code, symbol, date, expected string, now time.Time) (Observation, error) {
	var failure error
	if deps.Stock != nil {
		var rows []map[string]any
		var err error
		if key == "vkospi" {
			code, err = deps.Stock.ResolveVKOSPICode(ctx, nil)
			if err == nil {
				rows, err = deps.Stock.InquireVKOSPIDailyPrice(ctx, code, date)
			}
		} else {
			rows, err = deps.Stock.InquireIndexDailyPrice(ctx, code, date)
		}
		if err == nil {
			row := datedRow(rows, "stck_bsop_date", date, expected)
			o := rowNumber(row, "bstp_nmix_prpr", "stck_bsop_date", "KIS "+key+" daily", "points", 1, now)
			if o.available() && *o.Value > 0 {
				if diff, ok := num(row, "bstp_nmix_prdy_vrss"); ok && *o.Value-diff > 0 {
					o.PreviousClose = ptr(*o.Value - diff)
					o.ChangePct = ptr(diff / *o.PreviousClose * 100)
				}
				if pct, ok := num(row, "bstp_nmix_prdy_ctrt"); ok {
					o.ChangePct = ptr(pct)
				}
				return o, nil
			}
			failure = fmt.Errorf("no completed %s daily row matching %s before %s", key, expected, date)
		} else {
			failure = err
		}
	}
	if key == "vkospi" && deps.Naver != nil {
		h, err := deps.Naver.GetIndexDailyHistory(ctx, "VKOSPI", 20)
		if err == nil {
			rows := []map[string]any{}
			for _, bar := range h {
				rows = append(rows, map[string]any{"date": bar.Date, "price": bar.Close})
			}
			row := datedRow(rows, "date", date, expected)
			o := rowNumber(row, "price", "date", "Naver VKOSPI daily", "points", 1, now)
			if o.available() && *o.Value > 0 {
				prev := datedRow(rows, "date", o.BusinessDate, "")
				if v, ok := num(prev, "price"); ok && v > 0 {
					o.PreviousClose = ptr(v)
					o.ChangePct = ptr((*o.Value/v - 1) * 100)
					o.ReferenceDate = normalizeDate(parse.String(prev["date"]))
				}
				return o, nil
			}
		} else {
			failure = err
		}
	}
	if symbol != "" && deps.Yahoo != nil {
		h, err := deps.Yahoo.GetChartHistory(ctx, symbol, "1mo", "1d")
		if err == nil {
			rows := []map[string]any{}
			for _, bar := range h {
				rows = append(rows, map[string]any{"date": time.Unix(bar.DateUnix, 0).In(kst).Format("20060102"), "price": bar.Close})
			}
			row := datedRow(rows, "date", date, expected)
			o := rowNumber(row, "price", "date", "Yahoo "+symbol+" daily", "points", 1, now)
			if o.available() && *o.Value > 0 {
				prev := datedRow(rows, "date", o.BusinessDate, "")
				if v, ok := num(prev, "price"); ok && v > 0 {
					o.PreviousClose = ptr(v)
					o.ChangePct = ptr((*o.Value/v - 1) * 100)
					o.ReferenceDate = normalizeDate(parse.String(prev["date"]))
				}
				return o, nil
			}
		} else {
			failure = err
		}
	}
	return missing("NO_COMPLETED_DOMESTIC_ROW", now), failure
}

func normalizeDate(s string) string {
	s = strings.NewReplacer("-", "", ".", "", "/", "").Replace(strings.TrimSpace(s))
	if len(s) != 8 {
		return ""
	}
	if _, err := time.Parse("20060102", s); err != nil {
		return ""
	}
	return s
}
func datedRow(rows []map[string]any, key, before, exact string) map[string]any {
	var best map[string]any
	latest := ""
	for _, row := range rows {
		d := normalizeDate(parse.String(row[key]))
		if d == "" || d >= before || (exact != "" && d != exact) {
			continue
		}
		if d > latest {
			best, latest = row, d
		}
	}
	return best
}
func rowNumber(row map[string]any, key, dateKey, source, unit string, divisor float64, now time.Time) Observation {
	d := normalizeDate(parse.String(row[dateKey]))
	v, ok := num(row, key)
	if d == "" || !ok {
		return missing("MISSING_FIELD_OR_SOURCE_DATE", now)
	}
	return Observation{Value: ptr(v / divisor), Source: source, Unit: unit, BusinessDate: d, Session: "DAILY_CLOSE", Status: "VALID", FetchedAt: now}
}
