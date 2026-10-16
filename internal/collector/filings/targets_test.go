package filings_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fifi/internal/collector/filings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("공시 수집 대상 문서", func() {
	Context("watchlist 형식의 문서가 주어지면", func() {
		It("종목코드, 선택적 corp_code, 주석의 기업명을 읽는다", func() {
			doc := `# header comment
005930  # 삼성전자

000660 00164779 # SK하이닉스
# 012450  # 비활성 예시
0126z0
`
			targets, err := filings.ParseTargets(strings.NewReader(doc))
			Expect(err).NotTo(HaveOccurred())
			Expect(targets).To(HaveLen(3))
			Expect(targets[0]).To(Equal(filings.Target{StockCode: "005930", Name: "삼성전자", Line: 2}))
			Expect(targets[1]).To(Equal(filings.Target{StockCode: "000660", CorpCode: "00164779", Name: "SK하이닉스", Line: 4}))
			Expect(targets[2].StockCode).To(Equal("0126Z0"))
		})

		It("중복 종목코드는 처음 항목만 유지한다", func() {
			targets, err := filings.ParseTargets(strings.NewReader("005930 # A\n005930 # B\n"))
			Expect(err).NotTo(HaveOccurred())
			Expect(targets).To(HaveLen(1))
			Expect(targets[0].Name).To(Equal("A"))
		})
	})

	DescribeTable("형식이 잘못된 줄은 줄 번호와 함께 거부한다",
		func(doc, expected string) {
			_, err := filings.ParseTargets(strings.NewReader(doc))
			Expect(err).To(MatchError(ContainSubstring(expected)))
		},
		Entry("종목코드 길이 오류", "005930\n59300 # 짧음\n", "line 2: invalid stock code"),
		Entry("corp_code 형식 오류", "005930 1234 # 짧음\n", "line 1: invalid corp code"),
		Entry("필드 과다", "005930 00126380 extra\n", "line 1: expected"),
	)

	Context("저장소에 포함된 기본 문서는", func() {
		It("상위 10개 보통주를 형식 오류 없이 담고 있다", func() {
			targets, err := filings.LoadTargets(filepath.Join("..", "..", "..", filings.DefaultTargetsFile))
			Expect(err).NotTo(HaveOccurred())
			Expect(targets).To(HaveLen(10))
			Expect(targets[0].StockCode).To(Equal("005930"))
			for _, t := range targets {
				Expect(t.StockCode).NotTo(Equal("005935"), "preferred share must not be a separate target")
			}
		})
	})

	Describe("문서 경로 결정", func() {
		BeforeEach(func() {
			prev, had := os.LookupEnv(filings.TargetsFileEnvKey)
			DeferCleanup(func() {
				if had {
					os.Setenv(filings.TargetsFileEnvKey, prev)
				} else {
					os.Unsetenv(filings.TargetsFileEnvKey)
				}
			})
			os.Unsetenv(filings.TargetsFileEnvKey)
		})

		It("명시값, 환경변수, 기본값 순서로 사용한다", func() {
			Expect(filings.ResolveTargetsPath("")).To(Equal(filings.DefaultTargetsFile))
			os.Setenv(filings.TargetsFileEnvKey, "/tmp/env.txt")
			Expect(filings.ResolveTargetsPath("")).To(Equal("/tmp/env.txt"))
			Expect(filings.ResolveTargetsPath("custom.txt")).To(Equal("custom.txt"))
		})
	})
})

