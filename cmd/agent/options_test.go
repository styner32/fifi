package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent CLI Options", func() {
	Context("validateDate", func() {
		It("normalizes valid YYYY-MM-DD and YYYYMMDD dates", func() {
			Expect(validateDate("2026-07-05")).To(Equal("20260705"))
			Expect(validateDate("20260705")).To(Equal("20260705"))
			Expect(validateDate("")).To(Equal(""))
		})

		It("returns an error for invalid date formats", func() {
			_, err := validateDate("2026-07-0")
			Expect(err).To(HaveOccurred())

			_, err = validateDate("abcd-ef-gh")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("validateSidecar", func() {
		It("accepts valid sidecar status strings", func() {
			Expect(validateSidecar("triggered")).To(Succeed())
			Expect(validateSidecar("not-triggered")).To(Succeed())
			Expect(validateSidecar("unknown")).To(Succeed())
			Expect(validateSidecar("")).To(Succeed())
		})

		It("rejects invalid sidecar status strings", func() {
			Expect(validateSidecar("invalid-status")).To(HaveOccurred())
		})
	})

	Context("parseOptionalFloat", func() {
		It("parses optional floats and empty strings correctly", func() {
			v, err := parseOptionalFloat("", "test")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(BeNil())

			v, err = parseOptionalFloat("1,234.56", "test")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).NotTo(BeNil())
			Expect(*v).To(Equal(1234.56))
		})

		It("returns error for invalid float inputs", func() {
			_, err := parseOptionalFloat("invalid", "test")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("parseOptions", func() {
		It("parses CLI flags into snapshot options", func() {
			opts, err := parseOptions([]string{"--date", "2026-08-04", "--sidecar-status", "triggered", "--semiconductor-foreign-net-sell-eok", "1,200.5"})
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Date).To(Equal("20260804"))
			Expect(opts.SidecarStatus).To(Equal("triggered"))
			Expect(opts.SemiconductorForeignNetSellEok).NotTo(BeNil())
			Expect(*opts.SemiconductorForeignNetSellEok).To(Equal(1200.5))
		})
	})
})
