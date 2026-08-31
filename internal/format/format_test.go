package format

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Format Package Helpers", func() {
	Context("Number", func() {
		It("formats floats with specified decimal places and thousand separators", func() {
			Expect(Number(1234567.89, 2)).To(Equal("1,234,567.89"))
			Expect(Number(-1234567.89, 2)).To(Equal("-1,234,567.89"))
			Expect(Number(1234.5678, 3)).To(Equal("1,234.568"))
			Expect(Number(123, 0)).To(Equal("123"))
			Expect(Number(0.0, 1)).To(Equal("0.0"))
		})
	})

	Context("Signed", func() {
		It("adds explicit positive or negative signs", func() {
			Expect(Signed(123.4, 1)).To(Equal("+123.4"))
			Expect(Signed(-123.4, 1)).To(Equal("-123.4"))
			Expect(Signed(0, 1)).To(Equal("0.0"))
		})
	})

	Context("Percent and PercentPlain", func() {
		It("formats percent strings with and without signs", func() {
			Expect(Percent(1.234)).To(Equal("+1.23%"))
			Expect(Percent(-1.234)).To(Equal("-1.23%"))
			Expect(PercentPlain(1.234)).To(Equal("1.23%"))
		})
	})

	Context("Eok & TrillionFromEok", func() {
		It("formats eok and trillion values", func() {
			Expect(Eok(12.6)).To(Equal("+13"))
			Expect(Eok(-12.4)).To(Equal("-12"))
			Expect(TrillionFromEok(15000)).To(Equal("+1.5조원"))
		})
	})

	Context("Arrow and ArrowNeutral", func() {
		It("returns direction arrows based on sign", func() {
			Expect(Arrow(1.5)).To(Equal("▲+"))
			Expect(Arrow(-1.5)).To(Equal("▼"))
			Expect(Arrow(0)).To(Equal(" "))

			Expect(ArrowNeutral(1.5)).To(Equal("▲"))
			Expect(ArrowNeutral(-1.5)).To(Equal("▼"))
			Expect(ArrowNeutral(0)).To(Equal("─"))
		})
	})

	Context("EokArrow & AmountEok", func() {
		It("formats eok with direction arrows and units", func() {
			Expect(EokArrow(15000)).To(Equal("▲+15000억"))
			Expect(EokArrow(150)).To(Equal("▲+150억"))
			Expect(EokArrow(-15000)).To(Equal("▼-15000억"))
			Expect(EokArrow(-150)).To(Equal("▼-150억"))

			Expect(AmountEok(15000)).To(Equal("1.50조"))
			Expect(AmountEok(-150)).To(Equal("150억"))
		})
	})
})
