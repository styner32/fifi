package premarket

import (
	"context"
	"time"
)

func Collect(ctx context.Context, deps Deps, opts Options) *PremarketReport {
	now := time.Now()
	if deps.Clock != nil {
		now = deps.Clock()
	}

	date := now.Format("20060102")
	if opts.Date != "" {
		date = opts.Date
	}

	storeDir := opts.StoreDir
	if storeDir == "" {
		storeDir = deps.StoreDir
	}
	store := NewStore(storeDir)

	// Fetch current spot price approximations
	var kospiPrice, kosdaqPrice float64
	if deps.Stock != nil {
		if resp, err := deps.Stock.InquireIndexPrice(ctx, "0001"); err == nil && resp != nil {
			if r := firstRow(resp, "output"); r != nil {
				kospiPrice, _ = num(r, "bstp_nmix_prpr")
			}
		}
		if resp, err := deps.Stock.InquireIndexPrice(ctx, "1001"); err == nil && resp != nil {
			if r := firstRow(resp, "output"); r != nil {
				kosdaqPrice, _ = num(r, "bstp_nmix_prpr")
			}
		}
	}

	// Fetch VKOSPI
	var vkospiVal float64
	if deps.Stock != nil {
		if resp, err := deps.Stock.InquireVKOSPIPrice(ctx, "VKOSPI"); err == nil && resp != nil {
			if r := firstRow(resp, "output"); r != nil {
				vkospiVal, _ = num(r, "bstp_nmix_prpr")
			}
		}
	}

	// Tier 1: Directional Vector
	t1 := collectTier1(ctx, deps, kospiPrice, 1465.0)

	// Tier 2: Amplification Vector
	t2 := collectTier2(ctx, deps, store, kospiPrice, kosdaqPrice, vkospiVal, 1465.0, date)

	// Tier 3: Confirmation Vector
	t3 := collectTier3(ctx, -32000.0, 0.01)

	// Vulnerability Matrix
	vul := calculateVulnerabilityMatrix(t1, t2, t3)

	report := &PremarketReport{
		Timestamp: now,
		Date:      date,
		Tier1:     t1,
		Tier2:     t2,
		Tier3:     t3,
		VUL:       vul,
	}

	// Persist to store if save enabled
	if !opts.NoSave {
		store.UpsertRecord(DailyRecord{
			Date:                 date,
			KOSPIPrice:           kospiPrice,
			KOSDAQPrice:          kosdaqPrice,
			CreditLoanBalanceEok: t2.CreditLoanBalanceEok,
			MarginReceivableEok:  t2.MarginReceivableEok,
			ForcedSellAmountEok:  t2.ForcedSellAmountEok,
			CustomerDepositEok:   t2.CustomerDepositEok,
			VKOSPI:               t2.VKOSPI,
			SKHYADRClose:         t1.SKHYClose,
			USSemiComposite:      t1.SemiComposite,
			EchoEvents:           t2.EchoCalendar,
		})
		_ = store.Save()
	}

	return report
}
