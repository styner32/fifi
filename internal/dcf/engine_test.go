package dcf

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Value", func() {
	It("computes a valuation from projected cash flows", func() {
		fin := FinancialData{
			Revenue:      1000,
			EBIT:         180,
			EffectiveTax: 0.22,
			DnA:          40,
			CapEx:        50,
			ChangeInNWC:  10,
			SharesOut:    100,
			NetDebt:      200,
		}
		market := MarketData{
			RiskFreeRate:  0.03,
			Beta:          1.1,
			MarketPremium: 0.055,
			CostOfDebt:    0.045,
			EquityWeight:  0.7,
			DebtWeight:    0.3,
		}
		assumptions := Assumptions{ForecastYears: 5, TerminalGrowth: 0.02}
		projection := ProjectionModel{
			RevenueGrowth: 0.06,
			EBITMargin:    0.18,
			DNAMargin:     0.04,
			CapExMargin:   0.05,
			NWCMargin:     0.01,
		}

		result, err := Value(fin, market, assumptions, projection)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Forecast).To(HaveLen(5))
		Expect(result.BaseFCF).To(BeNumerically("~", 120.4, 1e-6))
		Expect(result.CostOfEquity).To(BeNumerically("~", 0.0905, 1e-6))
		Expect(result.WACC).To(BeNumerically("~", 0.07388, 1e-5))
		Expect(result.TargetPrice).To(BeNumerically(">", 0))
		Expect(result.TargetPriceRaw).To(BeNumerically(">", 0))
		Expect(result.TargetPriceScale).To(BeNumerically("==", defaultTargetPriceScale))
		Expect(result.TargetPriceUnit).To(Equal("KRW/share"))
		Expect(result.EnterpriseValue).To(BeNumerically(">", result.EquityValue))
		Expect(result.Forecast[0].Revenue).To(BeNumerically("~", 1060, 1e-6))
	})

	It("supports custom target price units for overseas equities", func() {
		fin := FinancialData{
			Revenue:      1000,
			EBIT:         180,
			EffectiveTax: 0.20,
			DnA:          30,
			CapEx:        35,
			ChangeInNWC:  10,
			SharesOut:    10,
			NetDebt:      -20,
		}
		market := MarketData{
			RiskFreeRate:  0.04,
			Beta:          1.2,
			MarketPremium: 0.05,
			CostOfDebt:    0.05,
			EquityWeight:  0.9,
			DebtWeight:    0.1,
		}

		result, err := Value(fin, market, Assumptions{
			ForecastYears:    5,
			TerminalGrowth:   0.025,
			TargetPriceScale: 1,
			TargetPriceUnit:  "USD/share",
		}, ProjectionModel{
			RevenueGrowth: 0.08,
			EBITMargin:    0.18,
			DNAMargin:     0.03,
			CapExMargin:   0.04,
			NWCMargin:     0.01,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.TargetPriceScale).To(BeNumerically("==", 1))
		Expect(result.TargetPriceUnit).To(Equal("USD/share"))
		Expect(result.TargetPrice).To(BeNumerically("==", result.TargetPriceRaw))
	})

	It("fails when wacc is not greater than terminal growth", func() {
		_, err := Value(
			FinancialData{Revenue: 100, EBIT: 10, EffectiveTax: 0.2, DnA: 3, CapEx: 4, ChangeInNWC: 1, SharesOut: 10},
			MarketData{RiskFreeRate: 0.01, Beta: 0.5, MarketPremium: 0.01, CostOfDebt: 0.01, EquityWeight: 0.8, DebtWeight: 0.2},
			Assumptions{ForecastYears: 5, TerminalGrowth: 0.02},
			ProjectionModel{RevenueGrowth: 0.03, EBITMargin: 0.1, DNAMargin: 0.03, CapExMargin: 0.04, NWCMargin: 0.01},
		)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("wacc"))
	})

	It("clamps extreme projection inputs", func() {
		model := normalizeProjectionModel(ProjectionModel{
			RevenueGrowth: 10,
			EBITMargin:    10,
			DNAMargin:     -1,
			CapExMargin:   10,
			NWCMargin:     -10,
		})

		Expect(model.RevenueGrowth).To(Equal(0.25))
		Expect(model.EBITMargin).To(Equal(0.60))
		Expect(model.DNAMargin).To(Equal(0.0))
		Expect(model.CapExMargin).To(Equal(0.40))
		Expect(model.NWCMargin).To(Equal(-0.20))
	})

	It("keeps the discount math stable", func() {
		fin := FinancialData{Revenue: 500, EBIT: 80, EffectiveTax: 0.2, DnA: 20, CapEx: 25, ChangeInNWC: 5, SharesOut: 50, NetDebt: 100}
		market := MarketData{RiskFreeRate: 0.03, Beta: 1.0, MarketPremium: 0.05, CostOfDebt: 0.04, EquityWeight: 0.75, DebtWeight: 0.25}
		assumptions := Assumptions{ForecastYears: 3, TerminalGrowth: 0.02}
		projection := ProjectionModel{RevenueGrowth: 0.04, EBITMargin: 0.16, DNAMargin: 0.04, CapExMargin: 0.05, NWCMargin: 0.01}

		result, err := Value(fin, market, assumptions, projection)
		Expect(err).NotTo(HaveOccurred())
		Expect(math.IsNaN(result.TargetPrice)).To(BeFalse())
		Expect(math.IsInf(result.TargetPrice, 0)).To(BeFalse())
	})
})
