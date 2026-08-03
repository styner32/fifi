package quadwitching

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WriteSnapshot", func() {
	It("writes a pretty JSON snapshot atomically", func() {
		path := filepath.Join(GinkgoT().TempDir(), "quad_witching_snapshot.json")
		payload := SnapshotExport{
			GeneratedAt:  "2026-03-12T15:30:00+09:00",
			BusinessDate: "20260312",
			FuturesCode:  "101V03",
			EndpointStates: map[string]EndpointSnapshot{
				"future_price": {Status: "ok", MessageCode: "0", Body: map[string]any{"futs_prpr": "362.35"}},
			},
		}

		Expect(WriteSnapshot(path, payload)).To(Succeed())

		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).To(ContainSubstring("\n  \"business_date\": \"20260312\""))

		var decoded SnapshotExport
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		Expect(decoded.FuturesCode).To(Equal("101V03"))
		Expect(decoded.EndpointStates).To(HaveKey("future_price"))
	})
})
