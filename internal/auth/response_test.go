package auth

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RESTResponse Rows and FirstRow", func() {
	Context("when response is nil or body is nil", func() {
		It("returns nil for Rows and FirstRow", func() {
			var resp *RESTResponse
			Expect(resp.Rows("output")).To(BeNil())
			Expect(resp.FirstRow("output")).To(BeNil())

			resp = &RESTResponse{}
			Expect(resp.Rows("output")).To(BeNil())
			Expect(resp.FirstRow("output")).To(BeNil())
		})
	})

	Context("when body contains rows", func() {
		var resp *RESTResponse

		BeforeEach(func() {
			resp = &RESTResponse{
				Body: map[string]any{
					"list_key": []any{
						map[string]any{"id": "1", "name": "foo"},
						map[string]any{"id": "2", "name": "bar"},
					},
					"map_key": map[string]any{
						"id":   "3",
						"name": "baz",
					},
					"string_key": "not-a-row",
				},
			}
		})

		It("extracts slice of maps from list_key", func() {
			rows := resp.Rows("list_key")
			Expect(rows).To(HaveLen(2))
			Expect(rows[0]["name"]).To(Equal("foo"))
			Expect(rows[1]["name"]).To(Equal("bar"))
		})

		It("wraps map_key in a slice", func() {
			rows := resp.Rows("map_key")
			Expect(rows).To(HaveLen(1))
			Expect(rows[0]["name"]).To(Equal("baz"))
		})

		It("returns nil for non-map/slice key", func() {
			Expect(resp.Rows("string_key")).To(BeNil())
			Expect(resp.Rows("missing")).To(BeNil())
		})

		It("finds first row using FirstRow", func() {
			row := resp.FirstRow("missing", "map_key")
			Expect(row).NotTo(BeNil())
			Expect(row["id"]).To(Equal("3"))

			row2 := resp.FirstRow("missing", "list_key")
			Expect(row2).NotTo(BeNil())
			Expect(row2["id"]).To(Equal("1"))

			Expect(resp.FirstRow("missing")).To(BeNil())
		})
	})
})
