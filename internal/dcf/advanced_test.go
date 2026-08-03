package dcf

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReverseDCF", func() {
	It("solves for the implied revenue growth that matches a target price", func() {
		fin := FinancialData{Revenue: 1000, EBIT: 180, EffectiveTax: 0.22, DnA: 40, CapEx: 50, ChangeInNWC: 10, SharesOut: 100, NetDebt: 200}
		market := MarketData{RiskFreeRate: 0.03, Beta: 1.1, MarketPremium: 0.055, CostOfDebt: 0.045, EquityWeight: 0.7, DebtWeight: 0.3}
		assumptions := Assumptions{ForecastYears: 5, TerminalGrowth: 0.02}
		projection := ProjectionModel{RevenueGrowth: 0.06, EBITMargin: 0.18, DNAMargin: 0.04, CapExMargin: 0.05, NWCMargin: 0.01}

		valuation, err := Value(fin, market, assumptions, projection)
		Expect(err).NotTo(HaveOccurred())

		result, err := ReverseDCF(fin, market, assumptions, projection, valuation.TargetPrice, ReverseDCFConfig{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.ImpliedRevenueGrowth).To(BeNumerically("~", 0.06, 1e-3))
		Expect(result.PriceError).To(BeNumerically("~", 0, 1e-3))
		Expect(result.Valuation).NotTo(BeNil())
	})

	It("reports the effective clamped growth range on bracket failure", func() {
		fin := FinancialData{Revenue: 1000, EBIT: 180, EffectiveTax: 0.22, DnA: 40, CapEx: 50, ChangeInNWC: 10, SharesOut: 100, NetDebt: 200}
		market := MarketData{RiskFreeRate: 0.03, Beta: 1.1, MarketPremium: 0.055, CostOfDebt: 0.045, EquityWeight: 0.7, DebtWeight: 0.3}
		assumptions := Assumptions{ForecastYears: 5, TerminalGrowth: 0.02}
		projection := ProjectionModel{RevenueGrowth: 0.06, EBITMargin: 0.18, DNAMargin: 0.04, CapExMargin: 0.05, NWCMargin: 0.01}

		_, err := ReverseDCF(fin, market, assumptions, projection, 999999999999, ReverseDCFConfig{LowerGrowth: -0.5, UpperGrowth: 1.0})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("effective growth range [-0.2000, 0.2500]"))
		Expect(err.Error()).To(ContainSubstring("requested [-0.5000, 1.0000]"))
	})
})

var _ = Describe("MonteCarlo", func() {
	It("produces a stable distribution summary", func() {
		fin := FinancialData{Revenue: 1000, EBIT: 180, EffectiveTax: 0.22, DnA: 40, CapEx: 50, ChangeInNWC: 10, SharesOut: 100, NetDebt: 200}
		market := MarketData{RiskFreeRate: 0.03, Beta: 1.1, MarketPremium: 0.055, CostOfDebt: 0.045, EquityWeight: 0.7, DebtWeight: 0.3}
		assumptions := Assumptions{ForecastYears: 5, TerminalGrowth: 0.02}
		projection := ProjectionModel{RevenueGrowth: 0.06, EBITMargin: 0.18, DNAMargin: 0.04, CapExMargin: 0.05, NWCMargin: 0.01}

		result, err := MonteCarlo(fin, market, assumptions, projection, MonteCarloConfig{Iterations: 2000, Workers: 4, Seed1: 1, Seed2: 2})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequestedIterations).To(Equal(2000))
		Expect(result.ValidIterations).To(BeNumerically(">", 1500))
		Expect(result.InvalidIterations).To(BeNumerically(">=", 0))
		Expect(result.Min).To(BeNumerically("<=", result.P10))
		Expect(result.P10).To(BeNumerically("<=", result.P50))
		Expect(result.P50).To(BeNumerically("<=", result.P90))
		Expect(result.P90).To(BeNumerically("<=", result.Max))
		Expect(result.Mean).To(BeNumerically(">", 0))
	})
})
