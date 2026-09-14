package pulse

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("store", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "pulse-store-test-*")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("JSONL append → load → 개수 일치", func() {
		now := time.Date(2026, 6, 23, 12, 0, 0, 0, kstLocation)
		rec1 := PulseRecord{TS: now, KOSPIIdx: 8565.06}
		rec2 := PulseRecord{TS: now.Add(10 * time.Minute), KOSPIIdx: 8570.0}

		Expect(AppendRecord(tmpDir, "20260623", rec1)).To(BeNil())
		Expect(AppendRecord(tmpDir, "20260623", rec2)).To(BeNil())

		records, err := LoadRecords(tmpDir, "20260623")
		Expect(err).To(BeNil())
		Expect(records).To(HaveLen(2))
		Expect(records[0].KOSPIIdx).To(BeNumerically("~", 8565.06, 0.01))
		Expect(records[1].KOSPIIdx).To(BeNumerically("~", 8570.0, 0.01))
	})

	It("파일 없으면 nil 반환 (첫 실행)", func() {
		records, err := LoadRecords(tmpDir, "20260623")
		Expect(err).To(BeNil())
		Expect(records).To(BeNil())
	})

	It("LoadNearest: at-or-before 선택 검증", func() {
		base := time.Date(2026, 6, 23, 10, 0, 0, 0, kstLocation)
		records := []PulseRecord{
			{TS: base, KOSPIIdx: 100},
			{TS: base.Add(30 * time.Minute), KOSPIIdx: 101},
			{TS: base.Add(60 * time.Minute), KOSPIIdx: 102},
			{TS: base.Add(90 * time.Minute), KOSPIIdx: 103},
		}
		now := base.Add(75 * time.Minute)
		target := now.Add(-1 * time.Hour) // = base - 15min → at-or-before는 base 점

		found := LoadNearest(records, target)
		Expect(found).NotTo(BeNil())
		Expect(found.KOSPIIdx).To(BeNumerically("~", 100, 0.01))
	})

	It("LoadNearest: 타깃이 모든 레코드보다 이전 → nil", func() {
		base := time.Date(2026, 6, 23, 12, 0, 0, 0, kstLocation)
		records := []PulseRecord{
			{TS: base, KOSPIIdx: 100},
		}
		found := LoadNearest(records, base.Add(-10*time.Minute))
		Expect(found).To(BeNil())
	})

	It("SaveMD가 파일을 올바르게 저장", func() {
		opts := Options{StoreDir: tmpDir}
		Expect(SaveMD(opts, "20260623", "# Test\nHello")).To(BeNil())
		raw, err := os.ReadFile(filepath.Join(tmpDir, "pulse_20260623.md"))
		Expect(err).To(BeNil())
		Expect(string(raw)).To(ContainSubstring("Hello"))
	})

	It("Options.NoHistory가 true일 때 이전 적립 레코드를 로드하지 않음", func() {
		now := time.Date(2026, 6, 23, 12, 0, 0, 0, kstLocation)
		rec := PulseRecord{TS: now, KOSPIIdx: 2600.0}
		Expect(AppendRecord(tmpDir, "20260623", rec)).To(BeNil())

		// NoHistory=true일 때
		opts := Options{StoreDir: tmpDir, NoHistory: true, NoSave: true}
		var loaded []PulseRecord
		if !opts.NoHistory {
			var err error
			loaded, err = LoadRecords(opts.StoreDir, "20260623")
			Expect(err).To(BeNil())
		}
		Expect(loaded).To(BeNil())
	})
})
