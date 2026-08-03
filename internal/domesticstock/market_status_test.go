package domesticstock

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KOSPIProxyPBR", func() {
	It("computes an 80 percent coverage proxy PBR with one master download and one calibration quote", func() {
		cacheDir := GinkgoT().TempDir()
		cachePath := filepath.Join(cacheDir, "kospi_code.mst")

		Expect(os.Setenv(kospiMasterCacheEnvKey, cachePath)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(kospiMasterCacheEnvKey)).To(Succeed())
		})

		masterBody := strings.Join([]string{
			buildKOSPIMasterLine("A000001", "ALPHA", "Y", "100", "20", "20250331", "5000"),
			buildKOSPIMasterLine("A000002", "BETA", "Y", "0", "0", "20250331", "1000"),
			buildKOSPIMasterLine("A000003", "GAMMA", "Y", "50", "20", "20250331", "2000"),
			buildKOSPIMasterLine("A000004", "DELTA", "Y", "20", "10", "20250331", "1000"),
			buildKOSPIMasterLine("A999999", "IGNORE", "N", "99", "10", "20250331", "9999"),
		}, "")

		zipBody, err := testhelpers.CreateMockZipArchive(kospiMasterFilename, []byte(masterBody))
		Expect(err).NotTo(HaveOccurred())

		transport := testhelpers.NewMockTransport()
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/kospi_code.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)

		transport.New("https://openapi.koreainvestment.com:9443").
			Get("/uapi/domestic-stock/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=J&FID_INPUT_ISCD=000001").
			Reply(http.StatusOK).
			MatchHeader("authorization", "Bearer test-token").
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "0",
				"msg1":   "정상처리 되었습니다.",
				"output": map[string]any{
					"pbr": "1.0",
				},
			})

		client := auth.NewKIClient("app", "secret", "https://openapi.koreainvestment.com:9443", "unit-test")
		client.Client = &http.Client{Transport: transport}
		client.SetAuthToken("test-token")

		svc := NewService(client)
		result, err := svc.KOSPIProxyPBR(context.Background(), 0.80)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Market).To(Equal("KOSPI"))
		Expect(result.Method).To(Equal("master-file-proxy"))
		Expect(result.TargetCoverage).To(BeNumerically("~", 0.80, 1e-9))
		Expect(result.EarningsScale).To(BeNumerically("~", 10.0, 1e-9))
		Expect(result.CalibrationSymbol).To(Equal("000001"))
		Expect(result.SelectedCount).To(Equal(4))
		Expect(result.UsedCount).To(Equal(3))
		Expect(result.SkippedCount).To(Equal(1))
		Expect(result.RawCoverage).To(BeNumerically("~", 1.0, 1e-9))
		Expect(result.UsedCoverage).To(BeNumerically("~", 8.0/9.0, 1e-9))
		Expect(result.ProxyPBR).To(BeNumerically("~", 8000.0/9500.0, 1e-9))
		Expect(result.Constituents).To(HaveLen(3))
		Expect(result.Constituents[0].Code).To(Equal("000001"))
		Expect(result.Constituents[1].Code).To(Equal("000003"))
		Expect(result.Constituents[2].Code).To(Equal("000004"))

		requests := transport.Requests()
		Expect(requests).To(HaveLen(2))
		Expect(requests[0].URL).To(ContainSubstring(kospiMasterFilename + ".zip"))
		Expect(requests[1].URL).To(ContainSubstring("FID_INPUT_ISCD=000001"))

		Expect(transport.Verify()).To(Succeed())
	})
})

var _ = Describe("KOSPIActualPBR", func() {
	It("uses cached actual pbr values and fetches only cache misses", func() {
		cacheDir := GinkgoT().TempDir()
		masterCachePath := filepath.Join(cacheDir, "kospi_code.mst")
		actualCacheBasePath := filepath.Join(cacheDir, "kospi_actual_pbr.json")
		actualCachePath := resolveKOSPIActualPBRCachePath(actualCacheBasePath, "20260309")

		Expect(os.Setenv(kospiMasterCacheEnvKey, masterCachePath)).To(Succeed())
		Expect(os.Setenv(kospiActualPBRCacheEnvKey, actualCacheBasePath)).To(Succeed())
		Expect(os.Setenv(kospiActualPBRRPMEnvKey, "0")).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(kospiMasterCacheEnvKey)).To(Succeed())
			Expect(os.Unsetenv(kospiActualPBRCacheEnvKey)).To(Succeed())
			Expect(os.Unsetenv(kospiActualPBRRPMEnvKey)).To(Succeed())
		})

		cached := actualPBRCache{
			"000001": {
				PBR:          2.0,
				MarketCap:    100.0,
				BusinessDate: "20260309",
				UpdatedAt:    "2026-03-09T09:00:00+09:00",
				Name:         "ALPHA",
			},
		}
		rawCache, err := json.Marshal(cached)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(actualCachePath, rawCache, 0o644)).To(Succeed())

		masterBody := strings.Join([]string{
			buildKOSPIMasterLine("A000001", "ALPHA", "Y", "100", "20", "20250331", "100"),
			buildKOSPIMasterLine("A000002", "BETA", "Y", "50", "20", "20250331", "50"),
			buildKOSPIMasterLine("A000003", "GAMMA", "Y", "20", "20", "20250331", "25"),
		}, "")

		zipBody, err := testhelpers.CreateMockZipArchive(kospiMasterFilename, []byte(masterBody))
		Expect(err).NotTo(HaveOccurred())

		transport := testhelpers.NewMockTransport()
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/kospi_code.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)

		transport.New("https://openapi.koreainvestment.com:9443").
			Get("/uapi/domestic-stock/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=J&FID_INPUT_ISCD=000002").
			Reply(http.StatusOK).
			MatchHeader("authorization", "Bearer test-token").
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "0",
				"msg1":   "정상처리 되었습니다.",
				"output": map[string]any{
					"pbr":      "1.0",
					"hts_avls": "50",
				},
			})

		client := auth.NewKIClient("app", "secret", "https://openapi.koreainvestment.com:9443", "unit-test")
		client.Client = &http.Client{Transport: transport}
		client.SetAuthToken("test-token")

		svc := NewService(client)
		result, err := svc.KOSPIActualPBR(context.Background(), 0.80, "20260309")
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Method).To(Equal("master-cap-actual-pbr"))
		Expect(result.TargetCoverage).To(BeNumerically("~", 0.80, 1e-9))
		Expect(result.ActualPBRCachePath).To(Equal(actualCachePath))
		Expect(result.CacheHitCount).To(Equal(1))
		Expect(result.APICallCount).To(Equal(1))
		Expect(result.SelectedCount).To(Equal(2))
		Expect(result.UsedCount).To(Equal(2))
		Expect(result.WeightedPBR).To(BeNumerically("~", 1.5, 1e-9))
		Expect(result.UsedCoverage).To(BeNumerically("~", 150.0/175.0, 1e-9))
		Expect(result.Constituents).To(HaveLen(2))
		Expect(result.Constituents[0].CacheHit).To(BeTrue())
		Expect(result.Constituents[1].CacheHit).To(BeFalse())

		requests := transport.Requests()
		Expect(requests).To(HaveLen(2))
		Expect(requests[1].URL).To(ContainSubstring("FID_INPUT_ISCD=000002"))

		savedCacheRaw, err := os.ReadFile(actualCachePath)
		Expect(err).NotTo(HaveOccurred())
		savedCache := actualPBRCache{}
		Expect(json.Unmarshal(savedCacheRaw, &savedCache)).To(Succeed())
		Expect(savedCache).To(HaveKey("000002"))
		Expect(savedCache["000002"].PBR).To(BeNumerically("~", 1.0, 1e-9))
		_, err = os.Stat(actualCacheBasePath)
		Expect(os.IsNotExist(err)).To(BeTrue())

		Expect(transport.Verify()).To(Succeed())
	})
})

var _ = Describe("loadKOSPIMaster", func() {
	It("separates master cache files by business date", func() {
		cacheDir := GinkgoT().TempDir()
		baseCachePath := filepath.Join(cacheDir, "kospi_code.mst")

		Expect(os.Setenv(kospiMasterCacheEnvKey, baseCachePath)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(kospiMasterCacheEnvKey)).To(Succeed())
		})

		masterBody := strings.Join([]string{
			buildKOSPIMasterLine("A000001", "ALPHA", "Y", "100", "20", "20250331", "5000"),
		}, "")

		zipBody, err := testhelpers.CreateMockZipArchive(kospiMasterFilename, []byte(masterBody))
		Expect(err).NotTo(HaveOccurred())

		transport := testhelpers.NewMockTransport()
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/kospi_code.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)
		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/kospi_code.mst.zip").
			Reply(http.StatusOK).
			Body(zipBody)

		client := auth.NewKIClient("app", "secret", "https://openapi.koreainvestment.com:9443", "unit-test")
		client.Client = &http.Client{Transport: transport}

		svc := NewService(client)

		records1, err := svc.loadKOSPIMaster(context.Background(), "20260309")
		Expect(err).NotTo(HaveOccurred())
		Expect(records1).To(HaveLen(1))

		records2, err := svc.loadKOSPIMaster(context.Background(), "20260309")
		Expect(err).NotTo(HaveOccurred())
		Expect(records2).To(HaveLen(1))

		records3, err := svc.loadKOSPIMaster(context.Background(), "20260310")
		Expect(err).NotTo(HaveOccurred())
		Expect(records3).To(HaveLen(1))

		cachePath0909 := resolveKOSPIMasterCachePath(baseCachePath, "20260309")
		cachePath0910 := resolveKOSPIMasterCachePath(baseCachePath, "20260310")
		jsonPath0909 := resolveKOSPIMasterJSONPath(cachePath0909)
		jsonPath0910 := resolveKOSPIMasterJSONPath(cachePath0910)

		Expect(cachePath0909).ToNot(Equal(cachePath0910))
		Expect(cachePath0909).To(BeAnExistingFile())
		Expect(cachePath0910).To(BeAnExistingFile())
		Expect(jsonPath0909).To(BeAnExistingFile())
		Expect(jsonPath0910).To(BeAnExistingFile())
		_, err = os.Stat(baseCachePath)
		Expect(os.IsNotExist(err)).To(BeTrue())

		rawJSON, err := os.ReadFile(jsonPath0909)
		Expect(err).NotTo(HaveOccurred())
		exported := kospiMasterJSONCache{}
		Expect(json.Unmarshal(rawJSON, &exported)).To(Succeed())
		Expect(exported.BusinessDate).To(Equal("20260309"))
		Expect(exported.RecordCount).To(Equal(1))
		Expect(exported.Fields).ToNot(BeEmpty())
		Expect(exported.Records).To(HaveLen(1))
		Expect(exported.Records[0].Code).To(Equal("000001"))

		requests := transport.Requests()
		Expect(requests).To(HaveLen(2))
		Expect(transport.Verify()).To(Succeed())
	})
})

func buildKOSPIMasterLine(
	code string,
	name string,
	kospiFlag string,
	netIncome string,
	roe string,
	baseDate string,
	marketCap string,
) string {
	fields := make([]string, len(kospiPart2FieldWidths))
	fields[58] = kospiFlag
	fields[62] = netIncome
	fields[63] = roe
	fields[64] = baseDate
	fields[65] = marketCap

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%-9s%-12s%s", code, "STD"+normalizeShortCode(code), name))
	for i, width := range kospiPart2FieldWidths {
		builder.WriteString(fmt.Sprintf("%-*s", width, fields[i]))
	}
	builder.WriteString("\n")
	return builder.String()
}
