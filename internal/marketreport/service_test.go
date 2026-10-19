package marketreport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/kst"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shared market reporting", func() {
	var config Config
	var now time.Time
	BeforeEach(func() {
		dir := GinkgoT().TempDir()
		now = time.Date(2026, 10, 15, 13, 0, 0, 0, kst.Location)
		config = Config{PremarketDir: filepath.Join(dir, "premarket"), PulseDir: filepath.Join(dir, "pulse"), SnapshotDir: filepath.Join(dir, "snapshots"), LockFile: filepath.Join(dir, "market.lock"), Clock: func() time.Time { return now }}
	})
	fixture := func(at time.Time) *snapshot.Snapshot {
		return &snapshot.Snapshot{BusinessDate: at.Format("20060102"), Timestamp: at, CompletedAt: at, SessionStatus: "INTRADAY_PROVISIONAL", Price: &snapshot.PriceSection{Date: at.Format("20060102"), Close: 2500, Status: "DATED_DAILY_OBSERVATION", MissingFields: []string{"trading_value_eok"}}, LateSession: &snapshot.LateSessionSection{Status: snapshot.StatusRawSpread, BasisAlignmentStatus: "RAW_SPREAD / ALIGNMENT_UNVERIFIED"}, Errors: map[string]error{"credit": errors.New("source unavailable")}}
	}
	Context("serialization and persistence", func() {
		It("preserves collector JSON including missing values and quality status", func() {
			report := fixture(now)
			service := NewWithCollectors(config, Collectors{Snapshot: func(_ context.Context, opts snapshot.Options) (*snapshot.Snapshot, error) {
				Expect(opts.Date).To(Equal("20261015"))
				return report, nil
			}})
			result, err := service.RunSnapshot(context.Background(), snapshot.Options{}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Envelope.SchemaVersion).To(Equal(1))
			Expect(result.Envelope.Artifacts).To(HaveLen(2))
			actual, err := os.ReadFile(filepath.Join(config.SnapshotDir, "market_snapshot.20261015.json"))
			Expect(err).NotTo(HaveOccurred())
			expected, err := json.Marshal(report.ToJSON())
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(MatchJSON(expected))
			envelopeJSON, err := json.Marshal(result.Envelope.Report)
			Expect(err).NotTo(HaveOccurred())
			Expect(envelopeJSON).To(MatchJSON(expected))
			Expect(result.Envelope.Warnings).To(ContainElement("credit: source unavailable"))
		})
		It("archives previous reports before atomically replacing the dated artifact", func() {
			counter := 0
			service := NewWithCollectors(config, Collectors{Snapshot: func(context.Context, snapshot.Options) (*snapshot.Snapshot, error) {
				counter++
				r := fixture(now)
				r.Price.Close += float64(counter)
				return r, nil
			}})
			_, err := service.RunSnapshot(context.Background(), snapshot.Options{}, true)
			Expect(err).NotTo(HaveOccurred())
			_, err = service.RunSnapshot(context.Background(), snapshot.Options{}, true)
			Expect(err).NotTo(HaveOccurred())
			archives, err := filepath.Glob(filepath.Join(config.SnapshotDir, "market_snapshot.20261015.previous.*.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(archives).To(HaveLen(1))
			var previous snapshot.SnapshotJSON
			raw, err := os.ReadFile(archives[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(raw, &previous)).To(Succeed())
			Expect(previous.Price.Close).To(Equal(2501.0))
			leftovers, err := filepath.Glob(filepath.Join(config.SnapshotDir, ".market-report-*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(leftovers).To(BeEmpty())
		})
		It("keeps no-save independent of reading pulse history", func() {
			service := NewWithCollectors(config, Collectors{Pulse: func(_ context.Context, opts pulse.Options) (*pulse.Pulse, error) {
				Expect(opts.NoSave).To(BeTrue())
				Expect(opts.NoHistory).To(BeFalse())
				return &pulse.Pulse{Date: "20261015", BusinessDate: "20261015", Now: now, KOSPI: pulse.Market{Index: pulse.IndexLevel{OK: true, Price: 2500}}, StoredCount: 5, Errors: map[string]string{}}, nil
			}})
			result, err := service.RunPulse(context.Background(), pulse.Options{NoSave: true})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Envelope.Artifacts).To(BeEmpty())
			Expect(result.Envelope.Report["stored_count"]).To(Equal(json.Number("5")))
			_, err = os.Stat(config.PulseDir)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
		It("saves premarket reports without inventing a missing history artifact", func() {
			service := NewWithCollectors(config, Collectors{Premarket: func(_ context.Context, opts premarket.Options) (*premarket.PremarketReport, error) {
				Expect(opts.NoSave).To(BeFalse())
				return &premarket.PremarketReport{SchemaVersion: 2, Timestamp: now, Date: "20261015", Inputs: map[string]premarket.Observation{"vix": {Value: new(float64(20)), Status: "VALID", Source: "fixture"}}, Errors: map[string]string{}}, nil
			}})
			result, err := service.RunPremarket(context.Background(), premarket.Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Envelope.Artifacts).To(HaveLen(2))
		})
	})
	Context("failure and concurrency", func() {
		DescribeTable("rejects invalid or future dates before collection", func(date string) {
			service := NewWithCollectors(config, Collectors{Snapshot: func(context.Context, snapshot.Options) (*snapshot.Snapshot, error) {
				Fail("collector must not be called")
				return nil, nil
			}})
			result, err := service.RunSnapshot(context.Background(), snapshot.Options{Date: date}, true)
			Expect(err).To(HaveOccurred())
			Expect(result.Envelope.Report).To(BeEmpty())
			Expect(result.Envelope.Artifacts).To(BeEmpty())
		}, Entry("invalid", "20260230"), Entry("future", "20261016"))
		It("returns complete partial diagnostics when no observations are usable", func() {
			service := NewWithCollectors(config, Collectors{Snapshot: func(context.Context, snapshot.Options) (*snapshot.Snapshot, error) {
				return &snapshot.Snapshot{BusinessDate: "20261015", Timestamp: now, Errors: map[string]error{"price": errors.New("upstream down")}}, nil
			}})
			result, err := service.RunSnapshot(context.Background(), snapshot.Options{}, true)
			Expect(err).To(HaveOccurred())
			Expect(result.Envelope.Report["errors"]).To(HaveKey("price"))
			Expect(result.Envelope.Artifacts).To(BeEmpty())
		})
		It("shares the lock across service instances and cancels waiting callers", func() {
			entered := make(chan struct{})
			release := make(chan struct{})
			done := make(chan error, 1)
			var calls atomic.Int32
			collectors := Collectors{Snapshot: func(ctx context.Context, _ snapshot.Options) (*snapshot.Snapshot, error) {
				calls.Add(1)
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return fixture(now), nil
			}}
			first := NewWithCollectors(config, collectors)
			second := NewWithCollectors(config, collectors)
			go func() { _, err := first.RunSnapshot(context.Background(), snapshot.Options{}, false); done <- err }()
			Eventually(entered).Should(BeClosed())
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err := second.RunSnapshot(ctx, snapshot.Options{}, false)
			Expect(err).To(MatchError(context.DeadlineExceeded))
			Expect(calls.Load()).To(Equal(int32(1)))
			close(release)
			Eventually(done).Should(Receive(BeNil()))
		})
		It("passes caller cancellation into the collector", func() {
			service := NewWithCollectors(config, Collectors{Snapshot: func(ctx context.Context, _ snapshot.Options) (*snapshot.Snapshot, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			_, err := service.RunSnapshot(ctx, snapshot.Options{}, true)
			Expect(err).To(MatchError(context.DeadlineExceeded))
		})
	})
})
