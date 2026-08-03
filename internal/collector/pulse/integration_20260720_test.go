package pulse

import (
	"os"
	"time"

	"github.com/fifi/internal/external/yahoo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("2026-07-20 Integration Test", func() {
	var originalKOSPIEnv string
	var originalKOSDAQEnv string

	BeforeEach(func() {
		originalKOSPIEnv = os.Getenv("OFFICIAL_EVENT_KOSPI_SIDECAR_SELL")
		originalKOSDAQEnv = os.Getenv("OFFICIAL_EVENT_KOSDAQ_SIDECAR_SELL")
		os.Setenv("OFFICIAL_EVENT_KOSPI_SIDECAR_SELL", "11:21:26")
		os.Setenv("OFFICIAL_EVENT_KOSDAQ_SIDECAR_SELL", "10:52:54")
	})

	AfterEach(func() {
		if originalKOSPIEnv != "" {
			os.Setenv("OFFICIAL_EVENT_KOSPI_SIDECAR_SELL", originalKOSPIEnv)
		} else {
			os.Unsetenv("OFFICIAL_EVENT_KOSPI_SIDECAR_SELL")
		}
		if originalKOSDAQEnv != "" {
			os.Setenv("OFFICIAL_EVENT_KOSDAQ_SIDECAR_SELL", originalKOSDAQEnv)
		} else {
			os.Unsetenv("OFFICIAL_EVENT_KOSDAQ_SIDECAR_SELL")
		}
	})

	Context("13:07 KST", func() {
		It("Sidecars are RELEASED (released after 5 minutes), N225 is HOLIDAY", func() {
			now1307 := time.Date(2026, 7, 20, 13, 7, 0, 0, kstLocation)
			kospiIdx := IndexLevel{Price: 2600.0, PrevClose: 2650.0, OK: true}
			kosdaqIdx := IndexLevel{Price: 800.0, PrevClose: 830.0, OK: true}
			k200 := IndexFutureSnapshot{Code: "101W7", Price: 350.0, PrevClose: 360.0, ChangePct: -2.7, SpotPrice: 351.0, Basis: -1.0, OK: true}
			kq150 := IndexFutureSnapshot{Code: "105W7", Price: 1200.0, PrevClose: 1250.0, ChangePct: -4.0, SpotPrice: 1205.0, Basis: -5.0, OK: true}

			safety := buildMarketSafety(now1307, "20260720", kospiIdx, kosdaqIdx, k200, kq150, nil)

			// Find KOSPI SIDECAR_SELL and KOSDAQ SIDECAR_SELL in safety.Devices
			var kospiSidecar, kosdaqSidecar *SafetyDeviceStatus
			for i := range safety.Devices {
				d := &safety.Devices[i]
				if d.Market == "KOSPI" && d.Device == "SIDECAR_SELL" {
					kospiSidecar = d
				}
				if d.Market == "KOSDAQ" && d.Device == "SIDECAR_SELL" {
					kosdaqSidecar = d
				}
			}

			Expect(kospiSidecar).NotTo(BeNil())
			Expect(kospiSidecar.State).To(Equal("RELEASED"))
			Expect(kospiSidecar.TriggeredAt).To(Equal("11:21:26"))
			Expect(kospiSidecar.ReleasedAt).To(Equal("11:26:26"))
			Expect(kospiSidecar.EligibleNow).To(BeFalse())

			Expect(kosdaqSidecar).NotTo(BeNil())
			Expect(kosdaqSidecar.State).To(Equal("RELEASED"))
			Expect(kosdaqSidecar.TriggeredAt).To(Equal("10:52:54"))
			Expect(kosdaqSidecar.ReleasedAt).To(Equal("10:57:54"))
			Expect(kosdaqSidecar.EligibleNow).To(BeFalse())

			// Nikkei 225 holiday test
			nikkeiLastTS := time.Date(2026, 7, 17, 15, 0, 0, 0, kstLocation) // Previous Friday close
			nikkeiWin := buildWindow("^N225", "닛케이225", yahoo.Quote{Price: 39500.0, ChangePercent: 0.0}, nil, now1307)
			Expect(nikkeiWin.Freshness).To(Equal("HOLIDAY"))
			Expect(nikkeiWin.Move1hPct).To(BeNil())
			Expect(nikkeiWin.Move2hPct).To(BeNil())
			Expect(nikkeiWin.Reason).To(ContainSubstring("휴장"))

			// Check freshness of nikkeiLastTS on holiday (it will identify as holiday)
			freshness, _, _ := DetermineFreshness("JPX", nikkeiLastTS, now1307, true)
			Expect(freshness).To(Equal("HOLIDAY"))
		})
	})

	Context("14:58 KST", func() {
		It("Sidecars and CB1/CB2 are EXPIRED_FOR_DAY", func() {
			now1458 := time.Date(2026, 7, 20, 14, 58, 0, 0, kstLocation)
			kospiIdx := IndexLevel{Price: 2600.0, PrevClose: 2650.0, OK: true}
			kosdaqIdx := IndexLevel{Price: 800.0, PrevClose: 830.0, OK: true}
			k200 := IndexFutureSnapshot{Code: "101W7", Price: 350.0, PrevClose: 360.0, ChangePct: -2.7, SpotPrice: 351.0, Basis: -1.0, OK: true}
			kq150 := IndexFutureSnapshot{Code: "105W7", Price: 1200.0, PrevClose: 1250.0, ChangePct: -4.0, SpotPrice: 1205.0, Basis: -5.0, OK: true}

			safety := buildMarketSafety(now1458, "20260720", kospiIdx, kosdaqIdx, k200, kq150, nil)

			// CB1, CB2, SIDECAR_SELL should be EXPIRED_FOR_DAY
			for i := range safety.Devices {
				d := &safety.Devices[i]
				if d.Device == "CB1" || d.Device == "CB2" {
					Expect(d.State).To(Equal("EXPIRED_FOR_DAY"))
					Expect(d.EligibleNow).To(BeFalse())
					Expect(d.EligibilityReason).To(ContainSubstring("발동 가능시간 종료"))
				}
				// SIDECAR_BUY is also expired since 14:58 > 14:50
				if d.Device == "SIDECAR_BUY" {
					Expect(d.State).To(Equal("EXPIRED_FOR_DAY"))
					Expect(d.EligibleNow).To(BeFalse())
					Expect(d.EligibilityReason).To(ContainSubstring("발동 가능시간 종료"))
				}
			}
		})
	})

	Context("15:24 KST", func() {
		It("Phase is CLOSING_AUCTION and Render output contains notice", func() {
			now1524 := time.Date(2026, 7, 20, 15, 24, 0, 0, kstLocation)
			p := &Pulse{
				Now:          now1524,
				Date:         "20260720",
				BusinessDate: "20260720",
				KOSPI: Market{
					Name:        "KOSPI",
					Index:       IndexLevel{Price: 2610.0, PrevClose: 2650.0, ChangePct: -1.5, OK: true},
					IntradayWin: Window{OK: true},
				},
				KOSDAQ: Market{
					Name:        "KOSDAQ",
					Index:       IndexLevel{Price: 805.0, PrevClose: 830.0, ChangePct: -3.0, OK: true},
					IntradayWin: Window{OK: true},
				},
				USDKRW: Window{Symbol: "KRW=X", Label: "원/달러", Current: 1350.0, ChangePct: 0.1, OK: true},
				Macro:  []Window{},
				Errors: map[string]string{},
			}

			out := Render(p)
			Expect(out).To(ContainSubstring("국내 거래소 세션 상태: CLOSING_AUCTION"))
			Expect(out).To(ContainSubstring("동시호가 진행 중"))
		})
	})
})
