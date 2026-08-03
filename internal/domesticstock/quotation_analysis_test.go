package domesticstock

import (
	"context"
	"net/http"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Quotation analysis", func() {
	It("fetches KOSPI investor daily flow by market", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/domestic-stock/v1/quotations/inquire-investor-daily-by-market?FID_COND_MRKT_DIV_CODE=U&FID_INPUT_DATE_1=20260515&FID_INPUT_DATE_2=20260515&FID_INPUT_ISCD=0001&FID_INPUT_ISCD_1=KSP&FID_INPUT_ISCD_2=0001").
			MatchHeader("authorization", "Bearer test-token").
			MatchHeader("tr_id", investorDailyByMarketTRID).
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd": "0", "msg_cd": "0", "msg1": "정상처리 되었습니다.",
				"output": []any{map[string]any{"frgn_ntby_tr_pbmn": "-4834200"}},
			})

		client.Client = &http.Client{Transport: transport}
		resp, err := NewService(client).InquireInvestorDailyByMarket(context.Background(), "20260515")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.IsOK()).To(BeTrue())
	})

	It("calculates 14-day RSI historical time-series correctly", func() {
		dates := []string{
			"20260701", "20260702", "20260703", "20260706", "20260707",
			"20260708", "20260709", "20260710", "20260713", "20260714",
			"20260715", "20260716", "20260717", "20260720", "20260721",
			"20260722", "20260723", "20260724", "20260727", "20260728",
		}
		closes := []float64{
			6500, 6520, 6490, 6530, 6550,
			6540, 6580, 6600, 6590, 6620,
			6650, 6630, 6680, 6700, 6670,
			6690, 6720, 6690, 6755, 6052,
		}

		series, err := CalculateRSISeries(dates, closes, 14)
		Expect(err).NotTo(HaveOccurred())
		Expect(series).To(HaveLen(6))
		Expect(series[0].Date).To(Equal("20260721"))
		Expect(series[0].RSI).To(BeNumerically(">", 0))
		Expect(series[5].Date).To(Equal("20260728"))
		Expect(series[5].Signal).NotTo(BeEmpty())
	})
})
