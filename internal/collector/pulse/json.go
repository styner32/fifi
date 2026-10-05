package pulse

import (
	"encoding/json"
	"strings"
)

// Keep legacy numeric Go fields for calculation, but serialize missing values as
// null. Consumers must inspect OK and source timing before using valid numbers.
func nullableJSON(v any, ok bool, extraNull ...string) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var m map[string]any
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, e
	}
	for k, v := range m {
		if x, ok := v.(string); ok && strings.HasPrefix(x, "0001-01-01T") {
			m[k] = nil
		}
	}
	if !ok {
		for k, v := range m {
			if _, numeric := v.(float64); numeric {
				m[k] = nil
			}
		}
	}
	for _, k := range extraNull {
		m[k] = nil
	}
	return json.Marshal(m)
}
func (f FlowSnapshot) MarshalJSON() ([]byte, error) {
	type alias FlowSnapshot
	var n []string
	if !f.EtcForeignOK {
		n = append(n, "EtcForeign")
	}
	return nullableJSON(alias(f), f.OK, n...)
}
func (i IndexLevel) MarshalJSON() ([]byte, error) {
	type alias IndexLevel
	var n []string
	for _, k := range i.MissingFields {
		if name, ok := map[string]string{"acml_tr_pbmn": "TradingValue", "bstp_nmix_oprc": "Open", "bstp_nmix_hgpr": "High", "bstp_nmix_lwpr": "Low"}[k]; ok {
			n = append(n, name)
		}
	}
	if !i.BreadthOK {
		n = append(n, "UpperLimit", "Advancers", "Unchanged", "Decliners", "LowerLimit", "TotalCount")
	}
	return nullableJSON(alias(i), i.OK, n...)
}
func (w Window) MarshalJSON() ([]byte, error) {
	type alias Window
	var n []string
	if !w.ChangeOK {
		n = append(n, "ChangePct", "prev_close")
	}
	return nullableJSON(alias(w), w.OK, n...)
}
func (f IndexFutureSnapshot) MarshalJSON() ([]byte, error) {
	type alias IndexFutureSnapshot
	var n []string
	if !f.MarketBasisOK {
		n = append(n, "market_basis", "basis_match")
	}
	if !f.SpotOK {
		n = append(n, "spot_price", "spot_change_pct", "raw_spread", "basis", "basis_match")
	}
	return nullableJSON(alias(f), f.OK, n...)
}
func (v VolatilitySnapshot) MarshalJSON() ([]byte, error) {
	type alias VolatilitySnapshot
	return nullableJSON(alias(v), v.OK)
}
func (v ProgramTradeSnapshot) MarshalJSON() ([]byte, error) {
	type alias ProgramTradeSnapshot
	return nullableJSON(alias(v), v.OK)
}
func (v PulseAssessment) MarshalJSON() ([]byte, error) {
	type alias PulseAssessment
	return nullableJSON(alias(v), true, "confidence")
}

func (v FlowDelta) MarshalJSON() ([]byte, error) {
	type alias FlowDelta
	var n []string
	if !v.EtcForeignOK {
		n = append(n, "EtcForeign")
	}
	if !v.IndexDeltaOK {
		n = append(n, "IndexDelta")
	}
	return nullableJSON(alias(v), true, n...)
}
func (v PulseRecord) MarshalJSON() ([]byte, error) {
	type alias PulseRecord
	var n []string
	if v.SchemaVersion >= 2 {
		if !v.KOSPIIndex.OK {
			n = append(n, "kospi_idx")
		}
		if !v.KOSDAQIndex.OK {
			n = append(n, "kosdaq_idx")
		}
		if !v.FX.OK {
			n = append(n, "usdkrw")
		}
	}
	return nullableJSON(alias(v), true, n...)
}
