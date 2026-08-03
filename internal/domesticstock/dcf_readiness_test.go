package domesticstock

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/marketpremium"
	"github.com/fifi/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DCFReadiness", func() {
	It("hydrates missing DCF inputs from KIS endpoints and assumptions", func() {
		transport := newDCFTransport()
		Expect(os.Setenv(dcfRiskFreeBondCodeEnvKey, "KR2033022D33")).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(dcfRiskFreeBondCodeEnvKey)).To(Succeed())
			Expect(os.Unsetenv(dcfRiskFreeBondDivEnvKey)).To(Succeed())
			Expect(os.Unsetenv(marketpremium.ProviderEnvKey)).To(Succeed())
			Expect(os.Unsetenv(dcfMarketPremiumEnvKey)).To(Succeed())
		})
		client := auth.NewKIClient("app", "secret", "https://openapi.koreainvestment.com:9443", "unit-test")
		client.Client = &http.Client{Transport: transport}
		client.SetAuthToken("test-token")

		svc := NewService(client)
		result, err := svc.DCFReadiness(context.Background(), "005930", "0")
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Symbol).To(Equal("005930"))
		Expect(result.Division).To(Equal("0"))
		Expect(result.BalancePeriods).To(Equal(2))
		Expect(result.IncomePeriods).To(Equal(2))
		Expect(result.CanProjectFCF).To(BeTrue())
		Expect(result.CanComputeWACC).To(BeTrue())
		Expect(result.CanComputeEnterpriseValue).To(BeTrue())
		Expect(result.CanComputeTargetPrice).To(BeTrue())
		Expect(result.MissingForFCF).To(BeEmpty())
		Expect(result.MissingForWACC).To(BeEmpty())
		Expect(result.MissingForTargetPrice).To(BeEmpty())

		inputs := map[string]DCFInputValue{}
		for _, input := range result.Inputs {
			inputs[input.Name] = input
		}

		Expect(inputs["Revenue"].Status).To(Equal(DCFInputExact))
		Expect(inputs["Revenue"].Value).To(BeNumerically("==", 3000))
		Expect(inputs["EffectiveTax"].Status).To(Equal(DCFInputDerived))
		Expect(inputs["EffectiveTax"].Value).To(BeNumerically("~", 0.20, 1e-9))
		Expect(inputs["CapEx"].Value).To(BeNumerically("==", 220))
		Expect(inputs["ChangeInNWC"].Value).To(BeNumerically("==", 50))
		Expect(inputs["NetDebt"].Status).To(Equal(DCFInputDerived))
		Expect(inputs["NetDebt"].Value).To(BeNumerically("~", 100, 1e-9))
		Expect(inputs["RiskFreeRate"].Status).To(Equal(DCFInputExact))
		Expect(inputs["RiskFreeRate"].Value).To(BeNumerically("~", 0.0285, 1e-9))
		Expect(inputs["Beta"].Status).To(Equal(DCFInputDerived))
		Expect(inputs["Beta"].Value).To(BeNumerically("~", 1.5, 1e-6))
		Expect(inputs["MarketPremium"].Status).To(Equal(DCFInputExact))
		Expect(inputs["MarketPremium"].Value).To(BeNumerically("~", 0.0417, 1e-9))
		Expect(inputs["CostOfDebt"].Status).To(Equal(DCFInputDerived))
		Expect(inputs["CostOfDebt"].Value).To(BeNumerically("~", 0.05, 1e-9))
		Expect(inputs["EquityWeight"].Value).To(BeNumerically("~", 3500.0/3600.0, 1e-9))
		Expect(inputs["DebtWeight"].Value).To(BeNumerically("~", 100.0/3600.0, 1e-9))

		Expect(transport.Verify()).To(Succeed())
	})
})

var _ = Describe("DCFValuation", func() {
	It("builds a full DCF valuation from domestic stock data", func() {
		transport := newDCFTransport()
		Expect(os.Setenv(dcfRiskFreeBondCodeEnvKey, "KR2033022D33")).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(dcfRiskFreeBondCodeEnvKey)).To(Succeed())
			Expect(os.Unsetenv(dcfRiskFreeBondDivEnvKey)).To(Succeed())
			Expect(os.Unsetenv(marketpremium.ProviderEnvKey)).To(Succeed())
			Expect(os.Unsetenv(dcfMarketPremiumEnvKey)).To(Succeed())
		})
		client := auth.NewKIClient("app", "secret", "https://openapi.koreainvestment.com:9443", "unit-test")
		client.Client = &http.Client{Transport: transport}
		client.SetAuthToken("test-token")

		svc := NewService(client)
		result, err := svc.DCFValuation(context.Background(), "005930", DCFValuationOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valuation).NotTo(BeNil())
		Expect(result.Financial.Revenue).To(BeNumerically("==", 3000))
		Expect(result.CurrentPrice).To(BeNumerically("==", 72000))
		Expect(result.Financial.NetDebt).To(BeNumerically("~", 100, 1e-9))
		Expect(result.Market.RiskFreeRate).To(BeNumerically("~", 0.0285, 1e-9))
		Expect(result.Market.MarketPremium).To(BeNumerically("~", 0.0417, 1e-9))
		Expect(result.Market.Beta).To(BeNumerically("~", 1.5, 1e-6))
		Expect(result.Valuation.WACC).To(BeNumerically(">", result.Assumptions.TerminalGrowth))
		Expect(result.Valuation.TargetPrice).To(BeNumerically(">", 0))
		Expect(result.Valuation.Forecast).To(HaveLen(defaultDCFForecastYears))

		Expect(transport.Verify()).To(Succeed())
	})
})

func newDCFTransport() *testhelpers.MockTransport {
	transport := testhelpers.NewMockTransport()
	baseURL := "https://openapi.koreainvestment.com:9443"
	lookbackFromDate := time.Now().AddDate(0, 0, -defaultDCFBetaLookback).Format("20060102")
	toDate := time.Now().Format("20060102")

	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=J&FID_INPUT_ISCD=005930").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":  "0",
			"msg_cd": "0",
			"msg1":   "정상처리 되었습니다.",
			"output": map[string]any{
				"lstn_stcn": "5969783",
				"hts_avls":  "3500",
				"stck_prpr": "72000",
			},
		})

	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/finance/balance-sheet?FID_DIV_CLS_CODE=0&fid_cond_mrkt_div_code=J&fid_input_iscd=005930").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":  "0",
			"msg_cd": "0",
			"msg1":   "정상처리 되었습니다.",
			"output": []any{
				map[string]any{"stac_yymm": "202412", "cras": "800", "fxas": "1200", "total_aset": "2000", "flow_lblt": "300", "fix_lblt": "400", "total_lblt": "700", "total_cptl": "1300"},
				map[string]any{"stac_yymm": "202312", "cras": "700", "fxas": "1100", "total_aset": "1800", "flow_lblt": "250", "fix_lblt": "350", "total_lblt": "600", "total_cptl": "1200"},
			},
		})

	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/finance/income-statement?FID_DIV_CLS_CODE=0&fid_cond_mrkt_div_code=J&fid_input_iscd=005930").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":  "0",
			"msg_cd": "0",
			"msg1":   "정상처리 되었습니다.",
			"output": []any{
				map[string]any{"stac_yymm": "202412", "sale_account": "3000", "depr_cost": "120", "bsop_prti": "500", "op_prfi": "450", "thtr_ntin": "360", "bsop_non_expn": "20"},
				map[string]any{"stac_yymm": "202312", "sale_account": "2800", "depr_cost": "110", "bsop_prti": "470", "op_prfi": "420", "thtr_ntin": "330", "bsop_non_expn": "18"},
			},
		})

	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/finance/other-major-ratios?fid_cond_mrkt_div_code=J&fid_div_cls_code=0&fid_input_iscd=005930").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":  "0",
			"msg_cd": "0",
			"msg1":   "정상처리 되었습니다.",
			"output": []any{
				map[string]any{"stac_yymm": "202412", "ebitda": "600", "ev_ebitda": "6"},
			},
		})

	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/finance/stability-ratio?fid_cond_mrkt_div_code=J&fid_div_cls_code=0&fid_input_iscd=005930").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":  "0",
			"msg_cd": "0",
			"msg1":   "정상처리 되었습니다.",
			"output": []any{
				map[string]any{"stac_yymm": "202412", "bram_depn": "20"},
			},
		})

	transport.New(baseURL).
		Get("/uapi/domestic-bond/v1/quotations/inquire-price?FID_COND_MRKT_DIV_CODE=B&FID_INPUT_ISCD=KR2033022D33").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":  "0",
			"msg_cd": "0",
			"msg1":   "정상처리 되었습니다.",
			"output": map[string]any{
				"hts_kor_isnm": "국고채 10년",
				"stnd_iscd":    "KR2033022D33",
				"ernn_rate":    "2.85",
			},
		})

	transport.New("https://pages.stern.nyu.edu").
		Get("/adamodar/New_Home_Page/home.htm").
		Reply(http.StatusOK).
		BodyString(`<html><body>Implied ERP on February 1, 2026 = 4.17% (Trailing 12 month, with adjusted payout); Est. year-end ERP = 4.40%</body></html>`)

	stockRows, indexRows := buildBetaRows(70)
	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice?FID_COND_MRKT_DIV_CODE=J&FID_INPUT_DATE_1="+lookbackFromDate+"&FID_INPUT_DATE_2="+toDate+"&FID_INPUT_ISCD=005930&FID_ORG_ADJ_PRC=1&FID_PERIOD_DIV_CODE=D").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":   "0",
			"msg_cd":  "0",
			"msg1":    "정상처리 되었습니다.",
			"output1": map[string]any{},
			"output2": stockRows,
		})

	transport.New(baseURL).
		Get("/uapi/domestic-stock/v1/quotations/inquire-index-daily-price?FID_COND_MRKT_DIV_CODE=U&FID_INPUT_DATE_1="+lookbackFromDate+"&FID_INPUT_ISCD=0001&FID_PERIOD_DIV_CODE=D").
		Reply(http.StatusOK).
		MatchHeader("authorization", "Bearer test-token").
		JSON(map[string]any{
			"rt_cd":   "0",
			"msg_cd":  "0",
			"msg1":    "정상처리 되었습니다.",
			"output1": map[string]any{},
			"output2": indexRows,
		})

	return transport
}

func buildBetaRows(length int) ([]any, []any) {
	stockRows := make([]any, 0, length)
	indexRows := make([]any, 0, length)
	stockClose := 100.0
	indexClose := 100.0
	for i := 0; i < length; i++ {
		date := fmt.Sprintf("2025%02d%02d", 1+(i/28), 1+(i%28))
		stockRows = append(stockRows, map[string]any{"stck_bsop_date": date, "stck_clpr": fmt.Sprintf("%.6f", stockClose)})
		indexRows = append(indexRows, map[string]any{"stck_bsop_date": date, "bstp_nmix_prpr": fmt.Sprintf("%.6f", indexClose)})
		indexReturn := 0.008
		if i%2 == 1 {
			indexReturn = -0.004
		}
		stockReturn := 1.5 * indexReturn
		stockClose *= 1 + stockReturn
		indexClose *= 1 + indexReturn
	}

	Expect(math.Abs(stockClose)).To(BeNumerically(">", 0))
	return stockRows, indexRows
}
