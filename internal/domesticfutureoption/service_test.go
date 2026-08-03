package domesticfutureoption

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("Service", func() {
	It("fetches domestic futures price", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/domestic-futureoption/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=F&FID_INPUT_ISCD=101V03").
			MatchHeader("authorization", "Bearer test-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "0",
				"msg1":   "정상처리 되었습니다.",
				"output1": map[string]any{
					"futs_prpr":      "362.35",
					"bstp_nmix_prpr": "361.10",
				},
			})

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		resp, err := svc.InquirePrice(context.Background(), "F", "101V03")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.IsOK()).To(BeTrue())
	})

	It("builds experimental time conclusion and member requests with futures params", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopccnl?FID_COND_MRKT_DIV_CODE=F&FID_INPUT_ISCD=101V03").
			MatchHeader("authorization", "Bearer test-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "0",
				"msg1":   "정상처리 되었습니다.",
				"output": []any{
					map[string]any{"stck_cntg_hour": "145959", "futs_prpr": "362.35"},
				},
			})

		transport.New("https://example.test").
			Get("/uapi/domestic-futureoption/v1/quotations/inquire-member?FID_COND_MRKT_DIV_CODE=F&FID_INPUT_ISCD=101V03").
			MatchHeader("authorization", "Bearer test-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "0",
				"msg1":   "정상처리 되었습니다.",
				"output": []any{
					map[string]any{"frgn_ntby_qty": "1200"},
				},
			})

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		respCCNL, err := svc.InquireTimeFuopCCNL(context.Background(), "F", "101V03")
		Expect(err).NotTo(HaveOccurred())
		Expect(respCCNL.IsOK()).To(BeTrue())

		respMember, err := svc.InquireMember(context.Background(), "F", "101V03")
		Expect(err).NotTo(HaveOccurred())
		Expect(respMember.IsOK()).To(BeTrue())
	})

	It("resolves near month KOSPI200 futures code from master data and writes dated caches", func() {
		cacheDir := GinkgoT().TempDir()
		baseCachePath := filepath.Join(cacheDir, "fo_idx_code_mts.mst")

		Expect(os.Setenv(indexFutureMasterCacheEnvKey, baseCachePath)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(indexFutureMasterCacheEnvKey)).To(Succeed())
			Expect(os.Unsetenv(quadWitchingFuturesCodeEnvKey)).To(Succeed())
		})

		masterBody := strings.Join([]string{
			"1|101V03|KR4101V03000|코스피200 선물 최근월| |00000000|1|201|KOSPI200",
			"1|101V06|KR4101V06000|코스피200 선물 차근월| |00000000|2|201|KOSPI200",
			"3|A06603|KR4306600000|코스닥150 선물 최근월| |00000000|1|3003|KSQ150",
			"7|701V03|KR4701V03000|변동성선물 최근월| |00000000|1|2050|VKOSPI",
		}, "\n")
		zipBody, err := testhelpers.CreateMockZipArchive(indexFutureMasterFilename, []byte(masterBody))
		Expect(err).NotTo(HaveOccurred())

		transport := testhelpers.NewMockTransport()
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/fo_idx_code_mts.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)

		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		resolved, err := svc.ResolveNearMonthKOSPI200Futures(context.Background(), "20260312")
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Source).To(Equal("master"))
		Expect(resolved.BusinessDate).To(Equal("20260312"))
		Expect(resolved.Record.ShortCode).To(Equal("101V03"))
		Expect(resolved.Record.MonthClassCode).To(Equal("1"))
		kosdaqResolved, err := svc.ResolveNearMonthKOSDAQ150Futures(context.Background(), "20260312")
		Expect(err).NotTo(HaveOccurred())
		Expect(kosdaqResolved.Record.ShortCode).To(Equal("A06603"))

		cachePath := resolveIndexFutureMasterCachePath(baseCachePath, "20260312")
		jsonPath := resolveIndexFutureMasterJSONPath(cachePath)
		Expect(cachePath).To(Equal(filepath.Join(cacheDir, "fo_idx_code_mts.20260312.mst")))
		Expect(resolved.MasterCachePath).To(Equal(cachePath))
		Expect(resolved.MasterJSONPath).To(Equal(jsonPath))
		_, err = os.Stat(cachePath)
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Stat(jsonPath)
		Expect(err).NotTo(HaveOccurred())

		Expect(transport.Verify()).To(Succeed())
	})

	It("prefers explicit futures code from environment", func() {
		Expect(os.Setenv(quadWitchingFuturesCodeEnvKey, "101V06")).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(quadWitchingFuturesCodeEnvKey)).To(Succeed())
		})

		svc := NewService(auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent"))
		resolved, err := svc.ResolveNearMonthKOSPI200Futures(context.Background(), "20260312")
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Source).To(Equal("env"))
		Expect(resolved.Record.ShortCode).To(Equal("101V06"))
	})
})
