package filings_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/fifi/internal/collector/filings"
	"github.com/fifi/internal/external/openai"
	"github.com/fifi/internal/kst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("공시 스냅샷 렌더링 및 파일 저장", func() {
	Context("OpenAI 분석 JSON이 주어지면", func() {
		It("DefaultReport 구조에서 요약, 사유, 재무 항목, 조건을 추출한다", func() {
			raw := json.RawMessage(`{
				"company_name": "LG에너지솔루션",
				"type": "영업(잠정)실적",
				"summary": "2026년 3분기 매출액 96,434억원, 영업이익 7,560억원 공시",
				"primary_cause": "3분기 잠정실적 공시",
				"data_extraction": {
					"financial_specifics": [
						{"item": "매출액", "value": "96,434", "unit": "억원", "details": "전년비 +59%"}
					],
					"conditions_terms": [
						{"term_name": "북미 보조금", "content": "AMPC 포함"}
					],
					"time_period": [
						{"label": "당기실적", "date": "2026-07-01~2026-09-30", "note": "3분기"}
					]
				}
			}`)

			entry := filings.ParseAnalysisJSON(raw)
			Expect(entry.CompanyName).To(Equal("LG에너지솔루션"))
			Expect(entry.ReportType).To(Equal("영업(잠정)실적"))
			Expect(entry.Summary).To(ContainSubstring("96,434억원"))
			Expect(entry.PrimaryCause).To(Equal("3분기 잠정실적 공시"))
			Expect(entry.FinancialSpecifics).To(HaveLen(1))
			Expect(entry.FinancialSpecifics[0].Item).To(Equal("매출액"))
			Expect(entry.ConditionsTerms).To(HaveLen(1))
			Expect(entry.TimePeriods).To(HaveLen(1))
		})
	})

	Context("공시 항목이 있는 스냅샷을 렌더링하면", func() {
		var snap *filings.Snapshot

		BeforeEach(func() {
			snap = &filings.Snapshot{
				Date:        "20261014",
				GeneratedAt: time.Date(2026, 10, 14, 17, 50, 0, 0, kst.Location),
				TargetsFile: "docs/filing_targets.txt",
				TargetCount: 10,
				FilingCount: 1,
				Entries: []filings.SnapshotEntry{
					{
						ReceiptNumber: "20261014800003",
						ReceiptDate:   "20261014",
						CorpCode:      "01515323",
						StockCode:     "373220",
						CompanyName:   "LG에너지솔루션",
						ReportName:    "조회공시요구(풍문또는보도)",
						ReportType:    "조회공시요구",
						Summary:       "전 경영진의 횡령·배임혐의 보도 관련 조회공시 요구",
						PrimaryCause:  "풍문 또는 보도 사실 여부 확인",
						FinancialSpecifics: []openai.FinancialSpecific{
							{Item: "공시시한", Value: "2026-10-14 18:00", Unit: "", Details: "당일 마감"},
						},
						Analyzed: true,
					},
				},
			}
		})

		It("요약표와 상세 분석 섹션을 포함한 마크다운을 생성한다", func() {
			md := filings.RenderSnapshot(snap)
			Expect(md).To(ContainSubstring("# DART 주요 기업 공시 스냅샷 — 20261014"))
			Expect(md).To(ContainSubstring("## 1. 당일 공시 요약"))
			Expect(md).To(ContainSubstring("LG에너지솔루션 (373220)"))
			Expect(md).To(ContainSubstring("20261014800003"))
			Expect(md).To(ContainSubstring("## 2. 공시별 상세 분석"))
			Expect(md).To(ContainSubstring("### [1] LG에너지솔루션 (373220) — 조회공시요구(풍문또는보도)"))
			Expect(md).To(ContainSubstring("**공시시한**: 2026-10-14 18:00 — 당일 마감"))
		})

		It("날짜별 MD·JSON 파일로 저장하고 재저장 시 이전 파일을 백업한다", func() {
			outDir := GinkgoT().TempDir()
			mdPath, jsonPath, err := filings.SaveSnapshotFiles(snap, outDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(mdPath).To(Equal(filepath.Join(outDir, "market_filings.20261014.md")))
			Expect(jsonPath).To(Equal(filepath.Join(outDir, "market_filings.20261014.json")))
			Expect(mdPath).To(BeAnExistingFile())
			Expect(jsonPath).To(BeAnExistingFile())

			// 두 번째 저장 시 .previous 백업 생성 확인
			_, _, err = filings.SaveSnapshotFiles(snap, outDir)
			Expect(err).NotTo(HaveOccurred())
			matches, err := filepath.Glob(filepath.Join(outDir, "market_filings.20261014.md.previous.*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).NotTo(BeEmpty())

			raw, err := os.ReadFile(mdPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).To(ContainSubstring("LG에너지솔루션"))
		})
	})

	Context("당일 접수된 공시가 없으면", func() {
		It("빈 안내 문구를 렌더링한다", func() {
			snap := &filings.Snapshot{
				Date:        "20261014",
				GeneratedAt: time.Date(2026, 10, 14, 17, 50, 0, 0, kst.Location),
				TargetCount: 10,
			}
			md := filings.RenderSnapshot(snap)
			Expect(md).To(ContainSubstring("2026-10-14 당일 접수된 주요 기업 공시가 없습니다."))
		})
	})
})

