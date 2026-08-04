package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
)

var _ = Describe("CMD Helpers", func() {
	Context("resolveBusinessDateFromMarketTime", func() {
		It("resolves business date when today matches", func() {
			resp := &auth.RESTResponse{
				Body: map[string]any{
					"output1": map[string]any{
						"today": "20260317",
						"date1": "20260317",
						"date2": "20260318",
						"date3": "20260319",
					},
				},
			}
			Expect(resolveBusinessDateFromMarketTime(resp, "20260316")).To(Equal("20260317"))
		})

		It("falls back to latest business date when today is not in options", func() {
			resp := &auth.RESTResponse{
				Body: map[string]any{
					"output1": map[string]any{
						"today": "20260317",
						"date1": "20260314",
						"date2": "20260316",
						"date3": "20260318",
					},
				},
			}
			Expect(resolveBusinessDateFromMarketTime(resp, "20260315")).To(Equal("20260316"))
		})
	})

	Context("Dated JSON path resolvers", func() {
		It("formats dated cache filenames correctly", func() {
			Expect(resolveMonteCarloJSONPath(".cache/dcf_monte_carlo.json", "20260317", "005930")).To(Equal(".cache/dcf_monte_carlo.20260317.005930.json"))
			Expect(resolveCompanyAnalysisJSONPath(".cache/company_analysis.json", "20260317", "NVDA")).To(Equal(".cache/company_analysis.20260317.NVDA.json"))
			Expect(resolveQuadWitchingSnapshotPath(".cache/quad_witching_snapshot.json", "20260317", "A01603")).To(Equal(".cache/quad_witching_snapshot.20260317.A01603.json"))
		})
	})

	Context("Env Default Helpers", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("CMD_TEST_STRING", "value")
			GinkgoT().Setenv("CMD_TEST_FLOAT", "1.25")
			GinkgoT().Setenv("CMD_TEST_INT", "7")
			GinkgoT().Setenv("CMD_TEST_BOOL", "true")
			GinkgoT().Setenv("CMD_TEST_OPTIONAL_FLOAT", "0.75")
		})

		It("retrieves environment variables with defaults", func() {
			Expect(getOrDefault("CMD_TEST_STRING", "fallback")).To(Equal("value"))
			Expect(getOrDefault("CMD_TEST_MISSING", "fallback")).To(Equal("fallback"))
			Expect(getFloatOrDefault("CMD_TEST_FLOAT", 0.5)).To(Equal(1.25))
			Expect(getIntOrDefault("CMD_TEST_INT", 3)).To(Equal(7))
			Expect(getBoolOrDefault("CMD_TEST_BOOL", false)).To(BeTrue())

			opt := getOptionalFloat("CMD_TEST_OPTIONAL_FLOAT")
			Expect(opt).NotTo(BeNil())
			Expect(*opt).To(Equal(0.75))

			optMissing := getOptionalFloat("CMD_TEST_OPTIONAL_FLOAT_MISSING")
			Expect(optMissing).To(BeNil())
		})
	})
})
