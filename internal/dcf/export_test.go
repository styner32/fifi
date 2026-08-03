package dcf

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WriteMonteCarloExport", func() {
	It("writes a pretty JSON payload atomically", func() {
		path := filepath.Join(GinkgoT().TempDir(), "dcf_monte_carlo.json")
		payload := MonteCarloExport{
			GeneratedAt:  "2026-03-10T12:00:00+09:00",
			BusinessDate: "20260310",
			Symbol:       "005930",
			CurrentPrice: 72000,
			MonteCarlo:   &MonteCarloResult{RequestedIterations: 100, ValidIterations: 100, Mean: 81234.5},
		}

		Expect(WriteMonteCarloExport(path, payload)).To(Succeed())

		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).To(ContainSubstring("\n  \"symbol\": \"005930\""))

		var decoded MonteCarloExport
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		Expect(decoded.Symbol).To(Equal("005930"))
		Expect(decoded.MonteCarlo.Mean).To(BeNumerically("==", 81234.5))
	})
})
