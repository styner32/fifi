package envcfg

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Envcfg Helpers", func() {
	Context("Get", func() {
		It("returns env value or default", func() {
			GinkgoT().Setenv("TEST_GET", "value")
			Expect(Get("TEST_GET", "default")).To(Equal("value"))
			Expect(Get("TEST_GET_MISSING", "default")).To(Equal("default"))
		})
	})

	Context("Float", func() {
		It("parses float env value or default", func() {
			GinkgoT().Setenv("TEST_FLOAT", "123.45")
			Expect(Float("TEST_FLOAT", 1.0)).To(Equal(123.45))
			Expect(Float("TEST_FLOAT_MISSING", 1.0)).To(Equal(1.0))
		})
	})

	Context("Int", func() {
		It("parses int env value or default", func() {
			GinkgoT().Setenv("TEST_INT", "100")
			Expect(Int("TEST_INT", 1)).To(Equal(100))
			Expect(Int("TEST_INT_MISSING", 1)).To(Equal(1))
		})
	})

	Context("Bool", func() {
		It("parses bool env value or default", func() {
			GinkgoT().Setenv("TEST_BOOL", "true")
			Expect(Bool("TEST_BOOL", false)).To(BeTrue())
			Expect(Bool("TEST_BOOL_MISSING", false)).To(BeFalse())
		})
	})

	Context("OptionalFloat", func() {
		It("parses optional float pointer or returns nil", func() {
			GinkgoT().Setenv("TEST_OPT_FLOAT", "45.67")
			opt := OptionalFloat("TEST_OPT_FLOAT")
			Expect(opt).NotTo(BeNil())
			Expect(*opt).To(Equal(45.67))

			optMissing := OptionalFloat("TEST_OPT_FLOAT_MISSING")
			Expect(optMissing).To(BeNil())
		})
	})
})
