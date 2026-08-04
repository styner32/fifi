package naver

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("Naver Client", func() {
	Context("GetIndexQuote", func() {
		It("parses realtime index quote correctly", func() {
			transport := testhelpers.NewMockTransport()
			transport.New(pollingBaseURL).
				Get("/api/realtime?query=SERVICE_INDEX:VKOSPI").
				Reply(http.StatusOK).
				BodyString(`{
					"resultCode": "success",
					"result": {
						"areas": [
							{
								"datas": [
									{
										"cd": "VKOSPI",
										"nv": 1550,
										"cv": 50,
										"cr": 3.33,
										"ov": 1500,
										"hv": 1600,
										"lv": 1490,
										"ms": "OPEN"
									}
								]
							}
						]
					}
				}`)

			client := NewClient(&http.Client{Transport: transport}, "test-ua")
			quote, err := client.GetIndexQuote(context.Background(), "VKOSPI")
			Expect(err).NotTo(HaveOccurred())
			Expect(quote.Price).To(Equal(15.5))
			Expect(quote.Change).To(Equal(0.5))
			Expect(quote.ChangePercent).To(Equal(3.33))
			Expect(quote.MarketStatus).To(Equal("OPEN"))
			Expect(transport.Verify()).To(Succeed())
		})
	})

	Context("GetIndexDailyHistory", func() {
		It("parses HTML sise index daily history in ascending order", func() {
			transport := testhelpers.NewMockTransport()
			transport.New(siseBaseURL).
				Get("/sise/sise_index_day.naver?code=KOSPI&page=1").
				Reply(http.StatusOK).
				BodyString(`
					<table>
						<tr class="item">
							<td class="date">2026.07.03</td>
							<td class="number_1">2,700.50</td>
						</tr>
						<tr class="item">
							<td class="date">2026.07.02</td>
							<td class="number_1">2,690.20</td>
						</tr>
					</table>
				`)

			client := NewClient(&http.Client{Transport: transport}, "test-ua")
			history, err := client.GetIndexDailyHistory(context.Background(), "KOSPI", 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(history).To(HaveLen(2))
			Expect(history[0].Date).To(Equal("2026.07.02"))
			Expect(history[0].Close).To(Equal(2690.2))
			Expect(history[1].Date).To(Equal("2026.07.03"))
			Expect(history[1].Close).To(Equal(2700.5))
		})
	})
})
