package overseasfuture

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/testhelpers"
)

var _ = Describe("Service", func() {
	It("fetches overseas future price by series code", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/overseas-futureoption/v1/quotations/inquire-price?SRS_CD=CLM26").
			MatchHeader("authorization", "Bearer test-token").
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd":  "0",
				"msg_cd": "00000",
				"msg1":   "정상처리 되었습니다.",
				"output1": map[string]any{
					"srs_cd": "CLM26",
					"last":   "71.25",
				},
			})

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		resp, err := svc.InquirePrice(context.Background(), "CLM26")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
		Expect(resp.IsOK()).To(BeTrue())
		Expect(resp.Body["output1"]).NotTo(BeNil())
	})

	It("resolves crude oil series code from master data", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		masterData := strings.Join([]string{
			buildMasterRow("CLN26", "NYMEX", "CL", false, true),
			buildMasterRow("CLQ26", "NYMEX", "CL", true, false),
			buildMasterRow("NGN26", "NYMEX", "NG", true, false),
		}, "\n")
		rawZip, err := testhelpers.CreateMockZipArchive("ffcode.mst", []byte(masterData))
		Expect(err).NotTo(HaveOccurred())

		transport.New("https://new.real.download.dws.co.kr").
			Get("/common/master/ffcode.mst.zip").
			Reply(http.StatusOK).
			Body(rawZip)

		client.Client = &http.Client{Transport: transport}
		svc := NewService(client)

		DeferCleanup(func() {
			Expect(os.Unsetenv(crudeOilSeriesCodeEnvKey)).To(Succeed())
			Expect(os.Unsetenv(crudeOilProductCodeEnvKey)).To(Succeed())
		})
		Expect(os.Unsetenv(crudeOilSeriesCodeEnvKey)).To(Succeed())
		Expect(os.Setenv(crudeOilProductCodeEnvKey, "CL")).To(Succeed())

		seriesCode, err := svc.ResolveCrudeOilSeriesCode(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(seriesCode).To(Equal("CLQ26"))
	})

	It("prefers explicit crude oil series code from environment", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		svc := NewService(client)

		DeferCleanup(func() {
			Expect(os.Unsetenv(crudeOilSeriesCodeEnvKey)).To(Succeed())
			Expect(os.Unsetenv(crudeOilProductCodeEnvKey)).To(Succeed())
		})
		Expect(os.Setenv(crudeOilSeriesCodeEnvKey, "BZM26")).To(Succeed())

		seriesCode, err := svc.ResolveCrudeOilSeriesCode(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(seriesCode).To(Equal("BZM26"))
	})

	It("normalizes yahoo style future tickers and reuses loaded records", func() {
		records := []MasterRecord{
			{SeriesCode: "HGH26", ExchangeCode: "CME", ProductCode: "HG", IsMostActive: false, IsRecent: true},
			{SeriesCode: "HGK26", ExchangeCode: "CME", ProductCode: "HG", IsMostActive: true, IsRecent: false},
			{SeriesCode: "SIH26", ExchangeCode: "CME", ProductCode: "SI", IsMostActive: false, IsRecent: true},
			{SeriesCode: "SIK26", ExchangeCode: "CME", ProductCode: "SI", IsMostActive: true, IsRecent: false},
		}

		Expect(NormalizeProductCode(" hg=f ")).To(Equal("HG"))
		Expect(NormalizeProductCode("si")).To(Equal("SI"))

		copperSeries, err := ResolveSeriesCodeByProductFromRecords(records, "HG=F")
		Expect(err).NotTo(HaveOccurred())
		Expect(copperSeries).To(Equal("HGK26"))

		silverSeries, err := ResolveSeriesCodeByProductFromRecords(records, "SI=F")
		Expect(err).NotTo(HaveOccurred())
		Expect(silverSeries).To(Equal("SIK26"))
	})
})

func buildMasterRow(seriesCode string, exchangeCode string, productCode string, mostActive bool, recent bool) string {
	row := bytesOfLength(200)

	putField(row, 0, 32, seriesCode)
	putField(row, len(row)-92, len(row)-82, exchangeCode)
	putField(row, len(row)-82, len(row)-72, productCode)
	putField(row, len(row)-7, len(row)-6, boolFlag(mostActive))
	putField(row, len(row)-6, len(row)-5, boolFlag(recent))
	putField(row, len(row)-5, len(row)-4, "0")

	return string(row)
}

func bytesOfLength(length int) []byte {
	row := make([]byte, length)
	for i := range row {
		row[i] = ' '
	}
	return row
}

func putField(row []byte, start int, end int, value string) {
	copy(row[start:end], fmt.Sprintf("%-*s", end-start, value))
}

func boolFlag(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
