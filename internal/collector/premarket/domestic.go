package premarket

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/parse"
)

// Optional capabilities avoid changing the contracts of pulse/snapshot clients.
type marketInvestor interface {
	InquireInvestorDailyForMarket(context.Context, string, string) (*auth.RESTResponse, error)
}
type stockDaily interface {
	InquireDailyItemChartPrice(context.Context, string, string, string, string, string) (*auth.RESTResponse, error)
	InquireInvestor(context.Context, string, string) (*auth.RESTResponse, error)
}

// Share raw responses with the existing price/foreign-flow table. One request per
// market/date ensures the displayed close, turnover and cumulative flow agree.
type dailyRowsResult struct {
	rows []map[string]any
	err  error
}
type flowResult struct {
	resp *auth.RESTResponse
	err  error
}
type memoStock struct {
	DomesticStock
	mu      sync.Mutex
	indices map[string]dailyRowsResult
	flows   map[string]flowResult
}

func (s *memoStock) InquireIndexDailyPrice(ctx context.Context, code, date string) ([]map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := code + date
	if r, ok := s.indices[key]; ok {
		return r.rows, r.err
	}
	rows, err := s.DomesticStock.InquireIndexDailyPrice(ctx, code, date)
	s.indices[key] = dailyRowsResult{rows, err}
	return rows, err
}
func (s *memoStock) InquireInvestorDailyByMarket(ctx context.Context, date string) (*auth.RESTResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.flows[date]; ok {
		return r.resp, r.err
	}
	r, e := s.DomesticStock.InquireInvestorDailyByMarket(ctx, date)
	s.flows[date] = flowResult{r, e}
	return r, e
}

func emptyDomestic(now time.Time) map[string]Observation {
	out := map[string]Observation{}
	for _, market := range []string{"kospi", "kosdaq"} {
		for _, investor := range []string{"foreign", "institution", "individual"} {
			for _, n := range []int{1, 3, 5} {
				out[fmt.Sprintf("%s_%s_%dd", market, investor, n)] = missing("DAILY_FLOW_UNAVAILABLE", now)
			}
		}
		for _, field := range []string{"advancers", "decliners", "unchanged", "advancing_pct", "turnover", "turnover_mean20", "turnover_ratio20"} {
			out[market+"_"+field] = missing("COMPLETED_DAILY_DATA_UNAVAILABLE", now)
		}
	}
	for _, code := range []string{"005930", "000660"} {
		for _, field := range []string{"close", "turnover", "foreign", "institution"} {
			out[code+"_"+field] = missing("STOCK_DAILY_DATA_UNAVAILABLE", now)
		}
	}
	return out
}

func collectDomestic(ctx context.Context, raw DomesticStock, memo *memoStock, date, prior string, now time.Time, errs map[string]string) map[string]Observation {
	out := emptyDomestic(now)
	if memo == nil || prior == "" {
		return out
	}
	for _, m := range []struct{ key, code, market string }{{"kospi", "0001", "KOSPI"}, {"kosdaq", "1001", "KOSDAQ"}} {
		rows, err := memo.InquireIndexDailyPrice(ctx, m.code, date)
		if err != nil {
			errs[m.key+"_history"] = err.Error()
		}
		ordered, err := orderedDailyRows(rows, date)
		if err != nil {
			errs[m.key+"_history"] = err.Error()
			ordered = nil
		}
		dates := []string{}
		for _, row := range ordered {
			d := parse.String(row["stck_bsop_date"])
			if d <= prior {
				dates = append(dates, d)
			}
		}
		row := datedRow(ordered, "stck_bsop_date", date, prior)
		source := "KIS " + m.market + " daily"
		out[m.key+"_turnover"] = nonnegative(rowNumber(row, "acml_tr_pbmn", "stck_bsop_date", source, "억원", 100, now), now)
		mean := missing("REQUIRES_20_PRECEDING_SESSIONS", now)
		if len(dates) >= 21 && dates[0] == prior {
			total, valid := 0.0, true
			for _, d := range dates[1:21] {
				r := datedRow(ordered, "stck_bsop_date", date, d)
				v, ok := num(r, "acml_tr_pbmn")
				if !ok || v < 0 {
					valid = false
					break
				}
				total += v / 100
			}
			if valid && total > 0 && finite(total) {
				mean = Observation{Value: ptr(total / 20), Unit: "억원", BusinessDate: prior, ReferenceDate: dates[20], Source: source, Session: "PRIOR_20_SESSION_MEAN", Status: "VALID", FetchedAt: now, SampleCount: 20, ReferenceEndDate: dates[1]}
			}
		}
		out[m.key+"_turnover_mean20"] = mean
		if turnover := out[m.key+"_turnover"]; turnover.available() && mean.available() {
			ratio := mean
			ratio.Value = ptr(*turnover.Value / *mean.Value)
			ratio.Unit = "배"
			ratio.Session = "DAILY_VS_PRIOR_20_MEAN"
			if finite(*ratio.Value) {
				out[m.key+"_turnover_ratio20"] = ratio
			}
		}
		var resp *auth.RESTResponse
		if m.market == "KOSPI" {
			resp, err = memo.InquireInvestorDailyByMarket(ctx, prior)
		} else if s, ok := raw.(marketInvestor); ok {
			resp, err = s.InquireInvestorDailyForMarket(ctx, m.market, prior)
		} else {
			err = fmt.Errorf("market daily investor capability unavailable")
		}
		if err != nil {
			errs[m.key+"_flows"] = err.Error()
			continue
		}
		if !responseOK(resp) {
			errs[m.key+"_flows"] = "invalid investor response"
			continue
		}
		flowRows, e := orderedDailyRows(resp.Rows("output"), date)
		if e != nil {
			errs[m.key+"_flows"] = e.Error()
			continue
		}
		for _, iv := range []struct{ key, field string }{{"foreign", "frgn_ntby_tr_pbmn"}, {"institution", "orgn_ntby_tr_pbmn"}, {"individual", "prsn_ntby_tr_pbmn"}} {
			for _, n := range []int{1, 3, 5} {
				key := fmt.Sprintf("%s_%s_%dd", m.key, iv.key, n)
				required := dates
				if n == 1 {
					required = []string{prior}
				}
				out[key] = sumFlows(flowRows, required, n, iv.field, date, prior, "KIS "+m.market+" investor daily", now)
			}
		}
	}
	if s, ok := raw.(stockDaily); ok {
		start, _ := time.Parse("20060102", prior)
		for _, code := range []string{"005930", "000660"} {
			resp, err := s.InquireDailyItemChartPrice(ctx, code, start.AddDate(0, 0, -14).Format("20060102"), prior, "D", "1")
			if err != nil {
				errs[code+"_price"] = err.Error()
			} else if responseOK(resp) {
				rows, e := orderedDailyRows(resp.Rows("output2"), date)
				if e != nil {
					errs[code+"_price"] = e.Error()
				} else {
					row := datedRow(rows, "stck_bsop_date", date, prior)
					close := rowNumber(row, "stck_clpr", "stck_bsop_date", "KIS KRX "+code+" daily", "KRW/share", 1, now)
					if close.available() && *close.Value > 0 {
						if diff, ok := signedDailyChange(row); ok && *close.Value-diff > 0 {
							close.PreviousClose = ptr(*close.Value - diff)
							close.ChangePct = ptr(diff / *close.PreviousClose * 100)
						}
						out[code+"_close"] = close
					}
					out[code+"_turnover"] = nonnegative(rowNumber(row, "acml_tr_pbmn", "stck_bsop_date", "KIS KRX "+code+" daily", "억원", 1e8, now), now)
				}
			}
			resp, err = s.InquireInvestor(ctx, "J", code)
			if err != nil {
				errs[code+"_flows"] = err.Error()
			} else if responseOK(resp) {
				rows, e := orderedDailyRows(resp.Rows("output"), date)
				if e != nil {
					errs[code+"_flows"] = e.Error()
					continue
				}
				for _, iv := range []struct{ key, field string }{{"foreign", "frgn_ntby_tr_pbmn"}, {"institution", "orgn_ntby_tr_pbmn"}} {
					out[code+"_"+iv.key] = sumFlows(rows, []string{prior}, 1, iv.field, date, prior, "KIS KRX "+code+" investor daily", now)
				}
			}
		}
	}
	return out
}

func signedDailyChange(row map[string]any) (float64, bool) {
	v, ok := num(row, "prdy_vrss")
	if !ok {
		return 0, false
	}
	switch parse.String(row["prdy_vrss_sign"]) {
	case "1", "2":
		return math.Abs(v), true
	case "4", "5":
		return -math.Abs(v), true
	case "3":
		return 0, v == 0
	default:
		return 0, false
	}
}
func nonnegative(o Observation, now time.Time) Observation {
	if o.available() && *o.Value < 0 {
		return missing("NEGATIVE_NONNEGATIVE_FIELD", now)
	}
	return o
}
func orderedDailyRows(rows []map[string]any, before string) ([]map[string]any, error) {
	byDate := map[string]map[string]any{}
	for _, r := range rows {
		d := normalizeDate(parse.String(r["stck_bsop_date"]))
		if d == "" || d >= before {
			continue
		}
		if prev, ok := byDate[d]; ok && !reflect.DeepEqual(prev, r) {
			return nil, fmt.Errorf("conflicting duplicate daily rows for %s", d)
		}
		copyRow := make(map[string]any, len(r))
		for k, v := range r {
			copyRow[k] = v
		}
		copyRow["stck_bsop_date"] = d
		byDate[d] = copyRow
	}
	dates := []string{}
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	out := []map[string]any{}
	for _, d := range dates {
		out = append(out, byDate[d])
	}
	return out, nil
}
func sumFlows(rows []map[string]any, dates []string, n int, field, before, prior, source string, now time.Time) Observation {
	o := missing(fmt.Sprintf("REQUIRES_%d_ALIGNED_SESSIONS", n), now)
	if len(dates) < n || dates[0] != prior {
		return o
	}
	total := 0.0
	for _, d := range dates[:n] {
		row := datedRow(rows, "stck_bsop_date", before, d)
		v, ok := num(row, field)
		if !ok {
			return o
		}
		total += v / 100
	}
	if !finite(total) {
		return missing("NONFINITE_FLOW_SUM", now)
	}
	return Observation{Value: ptr(total), Unit: "억원", BusinessDate: prior, ReferenceDate: dates[n-1], ReferenceEndDate: prior, SampleCount: n, Source: source, Session: "DAILY_FLOW", Status: "VALID", FetchedAt: now}
}
