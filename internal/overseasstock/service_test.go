package overseasstock

import (
	"context"
	"net/http"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("Service", func() {
	It("fetches overseas stock price by exchange code and symbol", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/overseas-price/v1/quotations/price?AUTH=&EXCD=AMS&SYMB=EWY").
			MatchHeader("authorization", "Bearer test-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "00000",
				"msg1":   "정상처리 되었습니다.",
				"output": map[string]any{
					"symb": "EWY",
					"last": "82.10",
				},
			})

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		resp, err := svc.Price(context.Background(), "AMS", "EWY")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.IsOK()).To(BeTrue())
		Expect(resp.Body["output"]).NotTo(BeNil())
	})

	It("fetches exchange-rate daily chart data", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/overseas-price/v1/quotations/inquire-daily-chartprice?FID_COND_MRKT_DIV_CODE=X&FID_INPUT_DATE_1=20260301&FID_INPUT_DATE_2=20260309&FID_INPUT_ISCD=USDKRW&FID_PERIOD_DIV_CODE=D").
			MatchHeader("authorization", "Bearer test-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "00000",
				"msg1":   "정상처리 되었습니다.",
				"output1": map[string]any{
					"hts_kor_isnm":        "미국달러/원",
					"ovrs_nmix_prpr":      "1330.50",
					"ovrs_nmix_prdy_clpr": "1328.00",
				},
				"output2": []map[string]any{
					{
						"stck_bsop_date": "20260309",
						"ovrs_nmix_prpr": "1330.50",
					},
				},
			})

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		resp, err := svc.InquireDailyChartPrice(context.Background(), "X", "USDKRW", "20260301", "20260309", "D")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.IsOK()).To(BeTrue())
		Expect(resp.Body["output1"]).NotTo(BeNil())
		Expect(resp.Body["output2"]).NotTo(BeNil())
	})

	It("resolves EWY exchange code from US stock master files", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		nasZip, err := testhelpers.CreateMockZipArchive("nasmst.cod", []byte("US\t1\tNAS\tNASDAQ\tAAPL\tAAPL\t애플\tAPPLE\n"))
		Expect(err).NotTo(HaveOccurred())
		nysZip, err := testhelpers.CreateMockZipArchive("nysmst.cod", []byte("US\t1\tNYS\tNYSE\tIBM\tIBM\tIBM\tIBM\n"))
		Expect(err).NotTo(HaveOccurred())
		amsZip, err := testhelpers.CreateMockZipArchive("amsmst.cod", []byte("US\t1\tAMS\tAMEX\tEWY\tEWY\t아이셰어즈 한국 ETF\tiShares MSCI South Korea ETF\n"))
		Expect(err).NotTo(HaveOccurred())

		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/nasmst.cod.zip").
			Reply(http.StatusOK).
			Body(nasZip)
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/nysmst.cod.zip").
			Reply(http.StatusOK).
			Body(nysZip)
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/amsmst.cod.zip").
			Reply(http.StatusOK).
			Body(amsZip)

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		DeferCleanup(func() {
			Expect(os.Unsetenv(ewyExchangeCodeEnvKey)).To(Succeed())
		})
		Expect(os.Unsetenv(ewyExchangeCodeEnvKey)).To(Succeed())

		exchangeCode, err := svc.ResolveEWYExchangeCode(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(exchangeCode).To(Equal("AMS"))
	})

	It("prefers explicit EWY exchange code from environment", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		svc := NewService(client)

		DeferCleanup(func() {
			Expect(os.Unsetenv(ewyExchangeCodeEnvKey)).To(Succeed())
		})
		Expect(os.Setenv(ewyExchangeCodeEnvKey, "AMS")).To(Succeed())

		exchangeCode, err := svc.ResolveEWYExchangeCode(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(exchangeCode).To(Equal("AMS"))
	})
})
