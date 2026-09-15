package premarket

import (
	"context"
	"time"
)

var kst = time.FixedZone("KST", 9*3600)

func Collect(ctx context.Context, deps Deps, opts Options) *PremarketReport {
	now := time.Now()
	if deps.Clock != nil {
		now = deps.Clock()
	}
	now = now.In(kst)
	date := now.Format("20060102")
	if opts.Date != "" {
		date = opts.Date
	}
	r := &PremarketReport{SchemaVersion: 2, Timestamp: now, Date: date, Errors: map[string]string{}}
	r.DomesticInputs = emptyDomestic(now)
	r.Calendar = emptyCalendar()
	rawStock := deps.Stock
	var memo *memoStock
	if rawStock != nil {
		memo = &memoStock{DomesticStock: rawStock, indices: map[string]dailyRowsResult{}, flows: map[string]flowResult{}}
		deps.Stock = memo
	}
	target, err := time.ParseInLocation("20060102", date, kst)
	if err != nil || target.After(now) {
		r.Errors["date"] = "invalid or future report date"
		r.Inputs = emptyInputs(now)
	} else {
		// Afternoon re-runs still select completed premarket cash sessions.
		cutoff := target.Add(9 * time.Hour)
		if now.Before(cutoff) {
			cutoff = now
		}
		r.CutoffAt = &cutoff
		r.Inputs, r.DomesticCloseDate = collectInputs(ctx, deps, date, cutoff, now, r.Errors)
		r.DomesticInputs = collectDomestic(ctx, rawStock, memo, date, r.DomesticCloseDate, now, r.Errors)
		collectBreadth(ctx, deps.DailyPrices, r.DomesticCloseDate, now, r.DomesticInputs, r.Errors)
		if deps.Calendar != nil {
			r.Calendar = collectCalendar(ctx, deps.Calendar, target, cutoff, now)
			r.Inputs["events"] = missing("EVENT_SCORING_NOT_IMPLEMENTED_CALENDAR_PARTIAL", now)
		}
	}
	dir := opts.StoreDir
	if dir == "" {
		dir = deps.StoreDir
	}
	store := NewStore(dir)
	if store.Err != nil {
		r.Errors["store_load"] = store.Err.Error()
	}
	r.Tier1 = collectTier1(r.Inputs)
	r.Tier2 = collectTier2(r.Inputs, store, date)
	r.Tier3 = collectTier3()
	r.VUL = calculateVulnerabilityMatrix(r.Tier1, r.Tier2, r.Inputs)
	r.HardData = collectHardData(r.Inputs)
	if !opts.NoSave && store.Err == nil && r.Errors["date"] == "" {
		store.UpsertRecord(DailyRecord{Date: date, ObservedAt: now, Inputs: r.Inputs, DomesticInputs: r.DomesticInputs, Calendar: &r.Calendar})
		if err := store.Save(); err != nil {
			r.Errors["store_save"] = err.Error()
		}
	}
	return r
}
