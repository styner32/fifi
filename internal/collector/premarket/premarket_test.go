package premarket_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/external/yahoo"
)

type mockYahoo struct{}

func (m *mockYahoo) GetQuotes(ctx context.Context, symbols []string) (map[string]yahoo.Quote, error) {
	out := map[string]yahoo.Quote{
		"SKHY":  {Symbol: "SKHY", Price: 154.57, ChangePercent: -7.47},
		"^SOX":  {Symbol: "^SOX", Price: 5120.0, ChangePercent: -2.23},
		"MU":    {Symbol: "MU", Price: 110.0, ChangePercent: -6.64},
		"NVDA":  {Symbol: "NVDA", Price: 120.0, ChangePercent: -4.99},
		"ASML":  {Symbol: "ASML", Price: 850.0, ChangePercent: -5.80},
		"NQ=F":  {Symbol: "NQ=F", Price: 28000.0, ChangePercent: -0.73},
		"EWY":   {Symbol: "EWY", Price: 65.0, ChangePercent: -2.50},
		"^VIX":  {Symbol: "^VIX", Price: 18.5, ChangePercent: 1.20},
		"KRW=X": {Symbol: "KRW=X", Price: 1468.0, ChangePercent: 0.15},
	}
	return out, nil
}

func (m *mockYahoo) GetChartHistory(ctx context.Context, symbol, rangeStr, intervalStr string) ([]yahoo.DailyClose, error) {
	return nil, nil
}

var _ = Describe("Premarket Vulnerability Indicator System", func() {
	var deps premarket.Deps
	var store *premarket.Store

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()
		store = premarket.NewStore(tmpDir)

		// Seed historical records for percentile & echo calendar
		store.UpsertRecord(premarket.DailyRecord{
			Date:                 "20260724",
			KOSPIPrice:           6690.62,
			CreditLoanBalanceEok: 335000,
			CustomerDepositEok:   1067000,
			VKOSPI:               77.55,
		})
		_ = store.Save()

		deps = premarket.Deps{
			Yahoo:    &mockYahoo{},
			Clock:    func() time.Time { return time.Date(2026, 7, 28, 7, 45, 0, 0, time.UTC) },
			StoreDir: tmpDir,
		}
	})

	It("calculates SKHY ADR Premium, SEMI_COMPOSITE, and DIVERGENCE correctly", func() {
		report := premarket.Collect(context.Background(), deps, premarket.Options{Date: "20260728", NoSave: true})

		Expect(report.Tier1.SKHYClose).To(Equal(154.57))
		Expect(report.Tier1.SKHYRet).To(Equal(-7.47))
		Expect(report.Tier1.SemiComposite).To(BeNumerically("<", -4.0))
		Expect(report.Tier1.Divergence).To(BeNumerically("<", -3.0))
		Expect(report.Tier1.HasDivAlert).To(BeTrue())
	})

	It("converts VKOSPI to daily sigma and evaluates VUL matrix grade", func() {
		report := premarket.Collect(context.Background(), deps, premarket.Options{Date: "20260728", NoSave: true})

		Expect(report.VUL.DScore).To(BeNumerically(">=", 2))
		Expect(report.VUL.OverallGrade).NotTo(BeEmpty())
		Expect(report.VUL.ConfidencePct).To(BeNumerically(">", 0))
	})

	It("renders Premarket board markdown without error", func() {
		report := premarket.Collect(context.Background(), deps, premarket.Options{Date: "20260728", NoSave: true})
		output := premarket.Render(report)

		Expect(output).To(ContainSubstring("개장 전 취약도 보드"))
		Expect(output).To(ContainSubstring("방향축 D"))
		Expect(output).To(ContainSubstring("크기축 A"))
		Expect(output).To(ContainSubstring("일정축 S"))
		Expect(output).To(ContainSubstring("취약도 종합"))
		Expect(output).To(ContainSubstring("포지션 사이징·시나리오 확률 사전 조정"))
	})
})
