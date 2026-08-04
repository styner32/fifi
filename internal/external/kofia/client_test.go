package kofia

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("KOFIA Client", func() {
	Context("GetMarketFunds", func() {
		It("fetches market funds from KOFIA web endpoint", func() {
			transport := testhelpers.NewMockTransport()
			transport.New(baseURL).
				Post("/meta/getMetaDataList.do").
				Reply(http.StatusOK).
				BodyString(`{
					"unit": "천원",
					"ds1": [
						{
							"TMPV1": "20260703",
							"TMPV2": 1000000000,
							"TMPV3": 200000000,
							"TMPV4": 3000000000,
							"TMPV5": 40000000,
							"TMPV6": 5000000,
							"TMPV7": 12.5
						}
					]
				}`)

			client := NewClient("test-ua")
			client.httpClient.Transport = transport

			rows, err := client.GetMarketFunds(context.Background(), "20260703", "20260703")
			Expect(err).NotTo(HaveOccurred())
			Expect(rows).To(HaveLen(1))

			row := rows[0]
			Expect(row.Date).To(Equal("20260703"))
			Expect(row.CustomerDepositMln).To(Equal(1000000.0))
			Expect(row.ForcedSellRatioPct).To(Equal(12.5))
			Expect(transport.Verify()).To(Succeed())
		})
	})

	Context("CachedClient GetMarketFundsForDate", func() {
		It("caches market funds on disk and uses cache hit on subsequent calls", func() {
			tmpDir := GinkgoT().TempDir()

			transport := testhelpers.NewMockTransport()
			transport.New(baseURL).
				Post("/meta/getMetaDataList.do").
				Reply(http.StatusOK).
				BodyString(`{
					"unit": "천원",
					"ds1": [
						{
							"TMPV1": "20260703",
							"TMPV2": 1000000000,
							"TMPV3": 200000000,
							"TMPV4": 3000000000,
							"TMPV5": 40000000,
							"TMPV6": 5000000,
							"TMPV7": 12.5
						}
					]
				}`)

			cc := NewCachedClient(tmpDir, "test-ua")
			cc.client.httpClient.Transport = transport

			// First load (cache miss, fetches from mock server)
			row1, err := cc.GetMarketFundsForDate(context.Background(), "20260703")
			Expect(err).NotTo(HaveOccurred())
			Expect(row1.CustomerDepositMln).To(Equal(1000000.0))
			Expect(transport.Verify()).To(Succeed())

			// Second load (cache hit, no request)
			transport.Reset()
			row2, err := cc.GetMarketFundsForDate(context.Background(), "20260703")
			Expect(err).NotTo(HaveOccurred())
			Expect(row2.CustomerDepositMln).To(Equal(1000000.0))
			Expect(transport.Requests()).To(BeEmpty())

			// Cache file exists
			cachePath := filepath.Join(tmpDir, "kofia_market_funds.20260703.json")
			_, err = os.Stat(cachePath)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
