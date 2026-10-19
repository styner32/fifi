package pulse

import (
	"context"
	"os"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/external/yahoo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type cancelDuringCollectionStock struct {
	DomesticStock
	cancel context.CancelFunc
}

func (s cancelDuringCollectionStock) MarketTime(ctx context.Context) (*auth.RESTResponse, error) {
	s.cancel()
	return nil, ctx.Err()
}
func (cancelDuringCollectionStock) InquireInvestorTimeByMarket(ctx context.Context, _, _ string) (*auth.RESTResponse, error) {
	return nil, ctx.Err()
}
func (cancelDuringCollectionStock) InquireIndexPrice(ctx context.Context, _ string) (*auth.RESTResponse, error) {
	return nil, ctx.Err()
}
func (cancelDuringCollectionStock) CompProgramTradeToday(ctx context.Context, _ string) (*auth.RESTResponse, error) {
	return nil, ctx.Err()
}
func (cancelDuringCollectionStock) ResolveVKOSPICode(ctx context.Context, _ []string) (string, error) {
	return "", ctx.Err()
}

type canceledFuture struct{ DomesticFuture }

func (canceledFuture) ResolveNearMonthKOSPI200Futures(ctx context.Context, _ string) (*domesticfutureoption.ResolvedContract, error) {
	return nil, ctx.Err()
}
func (canceledFuture) ResolveNearMonthKOSDAQ150Futures(ctx context.Context, _ string) (*domesticfutureoption.ResolvedContract, error) {
	return nil, ctx.Err()
}

type canceledYahoo struct{}

func (canceledYahoo) GetQuotes(ctx context.Context, _ []string) (map[string]yahoo.Quote, error) {
	return nil, ctx.Err()
}
func (canceledYahoo) GetChartHistory(ctx context.Context, _, _, _ string) ([]yahoo.DailyClose, error) {
	return nil, ctx.Err()
}

var _ = Describe("Pulse cancellation persistence", func() {
	Context("when an in-flight request is canceled", func() {
		It("preserves the existing JSONL and does not append an aborted actual-collector record", func() {
			dir := GinkgoT().TempDir()
			now := time.Date(2026, 10, 15, 13, 0, 0, 0, kstLocation)
			Expect(AppendRecord(dir, "20261015", PulseRecord{SchemaVersion: 2, BusinessDate: "20261015", TS: now.Add(-time.Hour), KOSPIIndex: IndexLevel{OK: true, Price: 2500, LastTS: now.Add(-time.Hour)}})).To(Succeed())
			path := pulseFilePath(dir, "20261015")
			before, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			report := Collect(ctx, Deps{Stock: cancelDuringCollectionStock{cancel: cancel}, Future: canceledFuture{}, Yahoo: canceledYahoo{}, Clock: func() time.Time { return now }}, Options{StoreDir: dir})
			Expect(report.Errors).To(HaveKeyWithValue("context", context.Canceled.Error()))
			Expect(report.Saved).To(BeFalse())
			after, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(before))
		})
	})
})
