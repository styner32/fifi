package filings_test

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fifi/internal/collector/filings"
	"github.com/fifi/internal/external/dart"
	"github.com/fifi/internal/kst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeCompanyLister struct {
	companies []dart.Company
	err       error
	calls     int
}

func (f *fakeCompanyLister) GetCompanies() ([]dart.Company, error) {
	f.calls++
	return f.companies, f.err
}

var _ = Describe("corp_code 매핑", func() {
	var (
		lister   *fakeCompanyLister
		resolver *filings.CorpCodeResolver
		base     string
		fixedNow time.Time
	)

	BeforeEach(func() {
		fixedNow = time.Date(2026, 10, 13, 9, 0, 0, 0, kst.Location)
		base = filepath.Join(GinkgoT().TempDir(), "dart_corp_code.json")
		lister = &fakeCompanyLister{companies: []dart.Company{
			{CorpCode: "00126380", CorpName: "삼성전자", StockCode: "005930"},
			{CorpCode: "00164779", CorpName: "SK하이닉스", StockCode: "000660"},
			{CorpCode: "00999999", CorpName: "비상장사", StockCode: " "},
		}}
		resolver = &filings.CorpCodeResolver{Lister: lister, CacheBasePath: base, Now: func() time.Time { return fixedNow }}
	})

	It("날짜가 포함된 캐시 경로를 만든다", func() {
		Expect(filings.ResolveCorpCodeCachePath(".cache/dart_corp_code.json", "20261013")).To(Equal(".cache/dart_corp_code.20261013.json"))
		Expect(filings.ResolveCorpCodeCachePath("", "20261013")).To(Equal(".cache/dart_corp_code.20261013.json"))
	})

	Context("종목코드만 있는 대상이면", func() {
		It("DART 목록으로 corp_code와 빈 기업명을 채우고, 찾지 못한 대상은 분리한다", func() {
			resolved, unresolved, err := resolver.Resolve([]filings.Target{
				{StockCode: "005930", Name: "삼성전자(문서)"},
				{StockCode: "000660"},
				{StockCode: "123456", Name: "없음"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved[0].CorpCode).To(Equal("00126380"))
			Expect(resolved[0].Name).To(Equal("삼성전자(문서)"))
			Expect(resolved[1].Name).To(Equal("SK하이닉스"))
			Expect(unresolved).To(HaveLen(1))
			Expect(unresolved[0].StockCode).To(Equal("123456"))
		})

		It("상장사만 날짜별 캐시에 저장하고 같은 날에는 다시 내려받지 않는다", func() {
			_, _, err := resolver.Resolve([]filings.Target{{StockCode: "005930"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(filings.ResolveCorpCodeCachePath(base, "20261013")).To(BeAnExistingFile())

			fresh := &filings.CorpCodeResolver{Lister: lister, CacheBasePath: base, Now: func() time.Time { return fixedNow }}
			resolved, _, err := fresh.Resolve([]filings.Target{{StockCode: "000660"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved[0].CorpCode).To(Equal("00164779"))
			Expect(lister.calls).To(Equal(1))
		})

		It("날짜가 바뀌면 새 캐시 파일을 만든다", func() {
			_, _, err := resolver.Resolve([]filings.Target{{StockCode: "005930"}})
			Expect(err).NotTo(HaveOccurred())
			fixedNow = fixedNow.AddDate(0, 0, 1)
			_, _, err = resolver.Resolve([]filings.Target{{StockCode: "005930"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(lister.calls).To(Equal(2))
			Expect(filings.ResolveCorpCodeCachePath(base, "20261014")).To(BeAnExistingFile())
		})
	})

	Context("모든 대상에 corp_code가 지정되어 있으면", func() {
		It("DART 목록을 내려받지 않는다", func() {
			lister.err = errors.New("must not be called")
			resolved, unresolved, err := resolver.Resolve([]filings.Target{{StockCode: "005930", CorpCode: "00126380"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(1))
			Expect(unresolved).To(BeEmpty())
			Expect(lister.calls).To(BeZero())
		})
	})

	Context("DART 목록 조회가 실패하고 캐시도 없으면", func() {
		It("오류를 반환한다", func() {
			lister.err = errors.New("network down")
			_, _, err := resolver.Resolve([]filings.Target{{StockCode: "005930"}})
			Expect(err).To(MatchError(ContainSubstring("network down")))
			_, statErr := os.Stat(filings.ResolveCorpCodeCachePath(base, "20261013"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})
	})
})

