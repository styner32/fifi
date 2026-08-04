package yahoo

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("Yahoo Client", func() {
	Context("GetQuotes", func() {
		It("fetches multiple market quotes concurrently", func() {
			transport := testhelpers.NewMockTransport()

			// Mock ^N225
			transport.New("https://example.test").
				Get("/v8/finance/chart/%5EN225?interval=1d&range=1d").
				MatchHeader("User-Agent", "unit-test").
				Reply(http.StatusOK).
				BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "^N225",
          "shortName": "Nikkei 225",
          "regularMarketPrice": 38500.25,
          "chartPreviousClose": 38600.25
        }
      }
    ],
    "error": null
  }
}`)

			// Mock NQ=F
			transport.New("https://example.test").
				Get("/v8/finance/chart/NQ%3DF?interval=1d&range=1d").
				MatchHeader("User-Agent", "unit-test").
				Reply(http.StatusOK).
				BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "NQ=F",
          "shortName": "Nasdaq 100 Futures",
          "regularMarketPrice": 18200.0,
          "chartPreviousClose": 18100.0
        }
      }
    ],
    "error": null
  }
}`)

			client := NewClient(&http.Client{Transport: transport}, Config{
				BaseURL:   "https://example.test",
				UserAgent: "unit-test",
			})
			quotes, err := client.GetQuotes(context.Background(), []string{"^N225", "NQ=F"})
			Expect(err).NotTo(HaveOccurred())
			Expect(quotes["^N225"].Price).To(Equal(38500.25))

			change := quotes["NQ=F"].ChangePercent
			Expect(change).To(BeNumerically(">=", 0.55))
			Expect(change).To(BeNumerically("<=", 0.56))
			Expect(transport.Verify()).To(Succeed())
		})

		It("returns partial quotes and error when some symbols fail", func() {
			transport := testhelpers.NewMockTransport()
			transport.New("https://example.test").
				Get("/v8/finance/chart/KRW%3DX?interval=1d&range=1d").
				Reply(http.StatusOK).
				BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "symbol": "KRW=X",
          "regularMarketPrice": 1494.2,
          "chartPreviousClose": 1490.0
        }
      }
    ],
    "error": null
  }
}`)

			// ^TNX will fail with 404
			transport.New("https://example.test").
				Get("/v8/finance/chart/%5ETNX?interval=1d&range=1d").
				Reply(http.StatusNotFound).
				BodyString(`Not Found`)

			client := NewClient(&http.Client{Transport: transport}, Config{BaseURL: "https://example.test"})
			quotes, err := client.GetQuotes(context.Background(), []string{"KRW=X", "^TNX"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("^TNX"))
			Expect(quotes).To(HaveKey("KRW=X"))
			Expect(transport.Verify()).To(Succeed())
		})
	})
})
