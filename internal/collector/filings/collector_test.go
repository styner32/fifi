package filings_test

import (
	"context"
	"errors"
	"time"

	"github.com/fifi/internal/collector/filings"
	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/fifi/internal/external/dart"
	"github.com/fifi/internal/kst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeFetcher struct {
	results map[string]tasks.FetchReportsResult
	errs    map[string]error
	calls   []dart.PageInfo
	onCall  func()
}

func (f *fakeFetcher) FetchAndAnalyzeReports(_ context.Context, pis ...dart.PageInfo) (tasks.FetchReportsResult, error) {
	f.calls = append(f.calls, pis...)
	if f.onCall != nil {
		f.onCall()
	}
	code := pis[0].CorpCode
	return f.results[code], f.errs[code]
}

type fakeLister struct {
	lists map[string][]dart.List
	calls []dart.PageInfo
}

func (f *fakeLister) GetRecentRawReports(pis ...dart.PageInfo) ([]dart.List, error) {
	f.calls = append(f.calls, pis...)
	return f.lists[pis[0].CorpCode], nil
}

var _ = Describe("공시 수집기", func() {
	var (
		fetcher  *fakeFetcher
		lister   *fakeLister
		c        *filings.Collector
		targets  []filings.Target
		fixedNow time.Time
	)

	BeforeEach(func() {
		fixedNow = time.Date(2026, 10, 13, 16, 0, 0, 0, kst.Location)
		fetcher = &fakeFetcher{results: map[string]tasks.FetchReportsResult{}, errs: map[string]error{}}
		lister = &fakeLister{lists: map[string][]dart.List{}}
		c = &filings.Collector{Fetcher: fetcher, Lister: lister, Now: func() time.Time { return fixedNow }, Logf: func(string, ...any) {}}
		targets = []filings.Target{
			{StockCode: "005930", CorpCode: "00126380", Name: "삼성전자"},
			{StockCode: "000660", CorpCode: "00164779", Name: "SK하이닉스"},
		}
	})

	Context("일반 모드에서", func() {
		It("기업마다 corp_code, 건수 제한, 조회 기간을 넘기고 결과를 합산한다", func() {
			c.Options = filings.Options{Limit: 3, Days: 7}
			fetcher.results["00126380"] = tasks.FetchReportsResult{Listed: 3, Stored: 2, Analyzed: 2, UsedTokens: 100}
			fetcher.results["00164779"] = tasks.FetchReportsResult{Listed: 3, Skipped: 3}

			sum := c.CollectOnce(context.Background(), targets)

			Expect(fetcher.calls).To(HaveLen(2))
			Expect(fetcher.calls[0].CorpCode).To(Equal("00126380"))
			Expect(fetcher.calls[0].Limit).To(Equal(3))
			Expect(fetcher.calls[0].StartDate.Format("20060102")).To(Equal("20261006"))
			Expect(fetcher.calls[0].EndDate.Format("20060102")).To(Equal("20261013"))
			Expect(sum.Failed).To(BeZero())
			Expect(sum.Stored).To(Equal(2))
			Expect(sum.Analyzed).To(Equal(2))
			Expect(sum.UsedTokens).To(Equal(int64(100)))
			Expect(lister.calls).To(BeEmpty())
		})

		It("옵션이 비어 있으면 기본 건수와 기간을 사용한다", func() {
			c.CollectOnce(context.Background(), targets[:1])
			Expect(fetcher.calls[0].Limit).To(Equal(filings.DefaultLimit))
			Expect(fetcher.calls[0].StartDate.Format("20060102")).To(Equal("20261008"))
		})

		It("한 기업이 실패해도 다음 기업을 계속 처리한다", func() {
			fetcher.errs["00126380"] = errors.New("dart timeout")
			fetcher.results["00164779"] = tasks.FetchReportsResult{Stored: 1, Analyzed: 1}

			sum := c.CollectOnce(context.Background(), targets)

			Expect(fetcher.calls).To(HaveLen(2))
			Expect(sum.Failed).To(Equal(1))
			Expect(sum.Companies[0].Err).To(MatchError("dart timeout"))
			Expect(sum.Companies[1].Err).NotTo(HaveOccurred())
			Expect(sum.Analyzed).To(Equal(1))
		})

		It("컨텍스트가 취소되면 남은 기업을 처리하지 않는다", func() {
			ctx, cancel := context.WithCancel(context.Background())
			fetcher.onCall = cancel

			sum := c.CollectOnce(ctx, targets)

			Expect(fetcher.calls).To(HaveLen(1))
			Expect(sum.Companies).To(HaveLen(1))
		})
	})

	Context("dry-run 모드에서", func() {
		It("공시 목록만 조회하고 저장·분석은 호출하지 않는다", func() {
			c.Options = filings.Options{DryRun: true}
			lister.lists["00126380"] = []dart.List{{RceptNo: "20261013000001", ReportNm: "기업설명회(IR)개최"}}

			sum := c.CollectOnce(context.Background(), targets)

			Expect(fetcher.calls).To(BeEmpty())
			Expect(lister.calls).To(HaveLen(2))
			Expect(sum.Companies[0].Disclosures).To(HaveLen(1))
			Expect(sum.Failed).To(BeZero())
		})

		It("목록 조회기가 없으면 실패로 기록한다", func() {
			c.Options = filings.Options{DryRun: true}
			c.Lister = nil
			sum := c.CollectOnce(context.Background(), targets[:1])
			Expect(sum.Failed).To(Equal(1))
		})
	})
})

