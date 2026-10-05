package kofia

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

// Record the request for assertions in the spec, without opening a test server.
type captureRequest struct{ search map[string]string }

func (c *captureRequest) RoundTrip(request *http.Request) (*http.Response, error) {
	var payload struct {
		Search map[string]string `json:"dmSearch"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		return nil, err
	}
	c.search = payload.Search
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"ds1":[{"TMPV1":"20260914","TMPV2":100,"TMPV3":100,"TMPV4":100,"TMPV5":100,"TMPV6":0,"TMPV7":0}]}`)),
		Request:    request,
	}, nil
}

var _ = Describe("KOFIA amount units and cache migration", func() {
	var ctx context.Context
	BeforeEach(func() { ctx = context.Background() })

	Context("when reading the captured million-KRW response", func() {
		var raw []byte
		BeforeEach(func() {
			var err error
			raw, err = os.ReadFile("testdata/market_funds_million_20260915.json")
			Expect(err).NotTo(HaveOccurred())
		})

		It("retains million-KRW amounts and reconciles the deposit scale with the same-date KIS value", func() {
			rows, err := decodeMarketFunds(raw)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rows)).To(BeNumerically(">=", 2))
			Expect(rows[0].AmountUnit).To(Equal("KRW_MILLION"))
			Expect(rows[0].MarginReceivableMln).To(Equal(986142.0))
			Expect(rows[0].ForcedSellAmountMln).To(Equal(10865.0))
			// KIS reports 1,070,560 eok for this date. The KOFIA response gives
			// 107,056,001 million KRW -> 1,070,560.01 eok, not ten times that value.
			Expect(rows[1].CustomerDepositMln / 100).To(BeNumerically("~", 1070560.01, 1e-7))
		})

		Context("when the dated cache has ambiguous legacy units", func() {
			var client *CachedClient
			var path string
			var original []byte
			BeforeEach(func() {
				client = NewCachedClient(GinkgoT().TempDir(), "")
				path = client.cachePath("20260915")
				original = []byte(`{"business_date":"20260915","rows":[{"date":"20260914","margin_receivable_mln":9861418.622}]}`)
				Expect(os.WriteFile(path, original, 0600)).To(Succeed())
			})

			It("rejects the legacy cache instead of treating its values as verified units", func() {
				_, err := client.loadCache(path)
				Expect(err).To(HaveOccurred())
			})

			It("fetches corrected units, preserves the original cache, and reuses the lagged result", func() {
				transport := testhelpers.NewMockTransport()
				transport.New(baseURL).Post("/meta/getMetaDataList.do").Reply(http.StatusOK).BodyString(string(raw))
				client.client.httpClient.Transport = transport

				By("refreshing from the response with an explicit amount unit")
				row, err := client.GetMarketFundsForDate(ctx, "20260915")
				Expect(err).NotTo(HaveOccurred())
				Expect(row).NotTo(BeNil())
				Expect(row.Date).To(Equal("20260914"))
				Expect(row.MarginReceivableMln).To(Equal(986142.0))
				Expect(transport.Verify()).To(Succeed())

				By("preserving the original cache bytes")
				archives, err := filepath.Glob(path + ".legacy.*")
				Expect(err).NotTo(HaveOccurred())
				Expect(archives).To(HaveLen(1))
				archived, err := os.ReadFile(archives[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(archived).To(Equal(original))

				By("reusing the corrected cache even though its reference date is lagged")
				transport.Reset()
				_, err = client.GetMarketFundsForDate(ctx, "20260915")
				Expect(err).NotTo(HaveOccurred())
				Expect(transport.Requests()).To(BeEmpty())
			})
		})
	})

	Context("when requesting market funds", func() {
		It("explicitly requests million KRW and retains a reported zero", func() {
			transport := &captureRequest{}
			client := NewClient("")
			client.httpClient.Transport = transport
			rows, err := client.GetMarketFunds(ctx, "20260914", "20260915")
			Expect(err).NotTo(HaveOccurred())
			Expect(transport.search).To(HaveKeyWithValue("tmpV40", "1000000"))
			Expect(rows).To(HaveLen(1))
			Expect(rows[0].ForcedSellAmountMln).To(BeZero())
		})
	})

	Context("when source fields cannot establish a valid amount", func() {
		DescribeTable("returns an error instead of manufacturing a value", func(raw string) {
			_, err := decodeMarketFunds([]byte(raw))
			Expect(err).To(HaveOccurred())
		},
			Entry("the amount unit is unknown", `{"unit":"unknown","ds1":[{"TMPV1":"20260914"}]}`),
			Entry("the forced-sale amount is missing", `{"ds1":[{"TMPV1":"20260914","TMPV2":100,"TMPV3":100,"TMPV4":100,"TMPV5":100,"TMPV7":0}]}`),
		)
	})
})
