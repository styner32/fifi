package commodityfuture

import (
	"context"
	"net/http"

	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service", func() {
	It("resolves gold and crude oil as known instruments", func() {
		gold := ResolveInstrument("Gold", "GC=F", "GC")
		Expect(gold.Name).To(Equal("Gold"))
		Expect(gold.CMEProductID).To(Equal("437"))
		Expect(gold.CMEPageURL).To(Equal("https://www.cmegroup.com/markets/metals/precious/gold.quotes.html"))
		Expect(gold.Exchange).To(Equal("COMEX"))

		crude := ResolveInstrument("", "CL=F", "CL")
		Expect(crude.Name).To(Equal("WTI Crude Oil"))
		Expect(crude.CMEProductID).To(Equal("425"))
		Expect(crude.CMEPageURL).To(Equal("https://www.cmegroup.com/markets/energy/crude-oil/light-sweet-crude.quotes.html"))
		Expect(crude.Exchange).To(Equal("NYMEX"))
	})

	It("uses the CME delayed quote endpoint when available", func() {
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://www.cmegroup.com").
			Get("/CmeWS/mvc/quotes/v2/438").
			MatchHeader("User-Agent", "test-agent").
			MatchHeader("Referer", "https://www.cmegroup.com/markets/metals/base/copper.quotes.html").
			Reply(http.StatusOK).
			BodyString(`{
  "quoteDelay": "600",
  "quotes": [
    {
      "quoteCode": "HGM26",
      "expirationMonth": "May 2026",
      "last": "5.735",
      "change": "-0.110",
      "percentageChange": "-1.88%",
      "priorSettle": "5.845",
      "open": "5.770",
      "high": "5.775",
      "low": "5.656",
      "volume": "14092",
      "updated": "2026-03-15T18:10:00Z",
      "exchangeCode": "COMEX",
      "isFrontMonth": true,
      "productName": "Copper Futures"
    }
  ]
}`)

		svc := NewService(&http.Client{Transport: transport}, Config{
			ProviderOrder: []string{ProviderCME},
			UserAgent:     "test-agent",
		})

		quote, err := svc.Quote(context.Background(), ResolveInstrument("Copper", "HG=F", "HG"))
		Expect(err).NotTo(HaveOccurred())
		Expect(quote).NotTo(BeNil())
		Expect(quote.Provider).To(Equal(ProviderCME))
		Expect(quote.ProviderPath).To(Equal("cme"))
		Expect(quote.Name).To(Equal("Copper"))
		Expect(quote.Contract).To(Equal("May 2026"))
		Expect(quote.QuoteCode).To(Equal("HGM26"))
		Expect(*quote.Price).To(BeNumerically("==", 5.735))
		Expect(*quote.Change).To(BeNumerically("~", -0.11, 0.0001))
		Expect(*quote.ChangePercent).To(BeNumerically("==", -1.88))
		Expect(quote.Delay).To(Equal("10 min"))
		Expect(quote.SourceURL).To(Equal("https://www.cmegroup.com/markets/metals/base/copper.quotes.html"))
	})

	It("falls back to Yahoo when CME blocks the request", func() {
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://www.cmegroup.com").
			Get("/CmeWS/mvc/quotes/v2/438").
			Reply(http.StatusOK).
			BodyString(`{"message":"This IP address is blocked due to suspected web scraping activity associated with it on this CMEgroup.com page."}`)

		transport.New("https://query1.finance.yahoo.com").
			Get("/v8/finance/chart/HG=F?interval=1d&range=5d").
			MatchHeader("User-Agent", "test-agent").
			Reply(http.StatusOK).
			BodyString(`{
  "chart": {
    "result": [
      {
        "meta": {
          "shortName": "Copper May 26",
          "fullExchangeName": "COMEX",
          "currency": "USD",
          "regularMarketTime": 1773600000,
          "regularMarketPrice": 5.735,
          "chartPreviousClose": 5.845,
          "regularMarketDayHigh": 5.775,
          "regularMarketDayLow": 5.656,
          "regularMarketVolume": 14092
        },
        "indicators": {
          "quote": [
            {
              "open": [5.770],
              "high": [5.775],
              "low": [5.656],
              "close": [5.735],
              "volume": [14092]
            }
          ]
        }
      }
    ],
    "error": null
  }
}`)

		svc := NewService(&http.Client{Transport: transport}, Config{
			ProviderOrder: []string{ProviderCME, ProviderYahoo},
			UserAgent:     "test-agent",
		})

		quote, err := svc.Quote(context.Background(), ResolveInstrument("Copper", "HG=F", "HG"))
		Expect(err).NotTo(HaveOccurred())
		Expect(quote).NotTo(BeNil())
		Expect(quote.Provider).To(Equal(ProviderYahoo))
		Expect(quote.ProviderPath).To(Equal("cme -> yahoo"))
		Expect(quote.Note).To(ContainSubstring("cme: blocked"))
		Expect(quote.Name).To(Equal("Copper May 26"))
		Expect(*quote.Price).To(BeNumerically("==", 5.735))
		Expect(*quote.PreviousClose).To(BeNumerically("==", 5.845))
		Expect(*quote.Change).To(BeNumerically("~", -0.11, 0.0001))
		Expect(*quote.ChangePercent).To(BeNumerically("~", -1.88195, 0.0001))
		Expect(*quote.Open).To(BeNumerically("==", 5.770))
		Expect(*quote.High).To(BeNumerically("==", 5.775))
		Expect(*quote.Low).To(BeNumerically("==", 5.656))
		Expect(*quote.Volume).To(BeNumerically("==", 14092))
		Expect(quote.ReferenceURL).To(Equal("https://www.cmegroup.com/markets/metals/base/copper.quotes.html"))
	})
})
