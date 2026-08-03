package quadwitching

import . "github.com/onsi/ginkgo/v2"
import . "github.com/onsi/gomega"

var _ = Describe("EvaluateRunWindow", func() {
	It("computes the second thursday schedule for quad witching months", func() {
		Expect(secondThursday(2026, 3).Format("20060102")).To(Equal("20260312"))
		Expect(secondThursday(2026, 6).Format("20060102")).To(Equal("20260611"))
		Expect(secondThursday(2026, 9).Format("20060102")).To(Equal("20260910"))
		Expect(secondThursday(2026, 12).Format("20060102")).To(Equal("20261210"))
	})

	It("runs when the business date is inside the lookahead window", func() {
		window, err := EvaluateRunWindow("20260305", 7, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(window.ShouldRun).To(BeTrue())
		Expect(window.QuadDate).To(Equal("20260312"))
		Expect(window.WindowStart).To(Equal("20260305"))
		Expect(window.WindowEnd).To(Equal("20260312"))
		Expect(window.DaysUntil).To(Equal(7))
	})

	It("skips when the business date is too early and points to the next quad witching day", func() {
		window, err := EvaluateRunWindow("20260304", 7, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(window.ShouldRun).To(BeFalse())
		Expect(window.QuadDate).To(Equal("20260312"))
		Expect(window.WindowStart).To(Equal("20260305"))
		Expect(window.DaysUntil).To(Equal(8))
	})

	It("supports a post-event grace window when configured", func() {
		window, err := EvaluateRunWindow("20260313", 7, 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(window.ShouldRun).To(BeTrue())
		Expect(window.QuadDate).To(Equal("20260312"))
		Expect(window.WindowEnd).To(Equal("20260313"))
		Expect(window.DaysUntil).To(Equal(-1))
	})

	It("switches to the next quarter after the grace window passes", func() {
		window, err := EvaluateRunWindow("20260313", 7, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(window.ShouldRun).To(BeFalse())
		Expect(window.QuadDate).To(Equal("20260611"))
		Expect(window.DaysUntil).To(Equal(90))
	})
})
