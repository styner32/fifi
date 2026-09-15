package premarket

import (
	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/parse"
	"math"
)

func collectTier2(in map[string]Observation, store *Store, date string) Tier2Amplification {
	t := Tier2Amplification{
		CreditLoanBalanceEok: in["credit"].Value, CustomerDepositEok: in["deposit"].Value,
		MarginReceivableEok: in["margin"].Value, ForcedSellAmountEok: in["forced"].Value,
		ForcedSellRatioPct: in["forced_ratio"].Value, VKOSPI: in["vkospi"].Value,
		AStatus: "NOT_EVALUATED", SStatus: "NOT_EVALUATED"}
	if in["vkospi"].available() {
		t.SigmaDaily = ptr(*t.VKOSPI / math.Sqrt(252))
		h := store.History("vkospi", in["vkospi"].BusinessDate, date, 250)
		t.VKOSPISampleCount = len(h)
		if len(h) == 250 {
			t.VKOSPIPctile250d = percentile(h, *t.VKOSPI)
		}
	}
	if in["credit"].available() {
		h := store.History("credit", in["credit"].BusinessDate, date, 60)
		t.CreditSampleCount = len(h)
		if len(h) == 60 {
			t.CreditLoanPctile = percentile(h, *t.CreditLoanBalanceEok)
		}
	}
	// The turnover, deposit-trend, margin-call and event models currently have
	// no verified implementation. Do not score A/S from their zero defaults.
	return t
}
func percentile(h []float64, current float64) *float64 {
	n := 0
	for _, v := range h {
		if v <= current {
			n++
		}
	}
	return ptr(float64(n) / float64(len(h)) * 100)
}
func firstRow(resp *auth.RESTResponse, keys ...string) map[string]any {
	if !responseOK(resp) {
		return nil
	}
	return resp.FirstRow(keys...)
}
func responseOK(resp *auth.RESTResponse) bool {
	if resp == nil || resp.Body == nil {
		return false
	}
	if code, ok := resp.Body["rt_cd"]; ok {
		return parse.String(code) == "0"
	}
	return true
}
func num(row map[string]any, keys ...string) (float64, bool) {
	v, ok := parse.Num(row, keys...)
	return v, ok && finite(v)
}
func finite(v float64) bool  { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func ptr(v float64) *float64 { return &v }
