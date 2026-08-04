package kst

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KST Time Location", func() {
	Context("Location", func() {
		It("is non-nil and has +9 hours UTC offset", func() {
			Expect(Location).NotTo(BeNil())
			now := time.Date(2026, 1, 1, 12, 0, 0, 0, Location)
			_, offset := now.Zone()
			Expect(offset).To(Equal(9 * 3600))
		})
	})

	Context("Now", func() {
		It("returns current time in KST location", func() {
			now := Now()
			Expect(now.Location()).To(Equal(Location))
		})
	})
})
