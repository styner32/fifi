package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/facts"
)

func backfillPulse(ctx context.Context, store facts.Store, dir string) (int, error) {
	stats, err := backfillPulseWithSummary(ctx, store, dir)
	return stats.AcceptedAttempts, err
}

func backfillPulseWithSummary(ctx context.Context, store facts.Store, dir string) (backfillSummary, error) {
	stats := backfillSummary{}
	files, err := filepath.Glob(filepath.Join(dir, "pulse_*.jsonl"))
	if err != nil {
		stats.fail("input_path_error", err)
		return stats, stats.Err()
	}
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			stats.fail("context_canceled", err)
			return stats, stats.Err()
		}
		f, err := os.Open(path)
		if err != nil {
			stats.Processed++
			stats.fail("input_read_error", err)
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 65536), 4*1024*1024)
		for scanner.Scan() {
			stats.Processed++
			if err := ctx.Err(); err != nil {
				_ = f.Close()
				stats.fail("context_canceled", err)
				return stats, stats.Err()
			}
			var header struct {
				SchemaVersion int `json:"schema_version"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
				stats.fail("invalid_json", fmt.Errorf("invalid pulse JSON in %s", filepath.Base(path)))
				continue
			}
			if header.SchemaVersion != 2 {
				stats.skip(schemaSkipReason(header.SchemaVersion))
				continue
			}
			var rec pulse.PulseRecord
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				stats.fail("invalid_json", fmt.Errorf("invalid pulse JSON in %s", filepath.Base(path)))
				continue
			}
			var raw map[string]any
			_ = json.Unmarshal(scanner.Bytes(), &raw)
			if obs, reason := classifiedPulseObservation(rec, raw, time.Now()); reason == "" {
				if err := store.Put(ctx, obs); err != nil {
					stats.fail("store_write_error", err)
				} else {
					stats.AcceptedAttempts++
				}
			} else {
				stats.skip(reason)
			}
		}
		if err := scanner.Err(); err != nil {
			stats.Processed++
			stats.fail("input_read_error", err)
		}
		_ = f.Close()
	}
	return stats, stats.Err()
}

func pulseObservation(rec pulse.PulseRecord, raw map[string]any, now time.Time) (facts.Observation, bool) {
	obs, reason := classifiedPulseObservation(rec, raw, now)
	return obs, reason == ""
}

func classifiedPulseObservation(rec pulse.PulseRecord, raw map[string]any, now time.Time) (facts.Observation, string) {
	v := rec.VKOSPI
	if rec.SchemaVersion != 2 {
		return facts.Observation{}, schemaSkipReason(rec.SchemaVersion)
	}
	if !v.OK {
		return facts.Observation{}, "source_status_unusable"
	}
	if v.Source == "" {
		return facts.Observation{}, "source_missing"
	}
	if v.LastTS.IsZero() || rec.TS.IsZero() {
		return facts.Observation{}, "source_timestamp_missing"
	}
	if v.Value <= 0 || math.IsNaN(v.Value) || math.IsInf(v.Value, 0) {
		return facts.Observation{}, "invalid_value"
	}
	switch v.Freshness {
	case "FRESH", "DELAYED", "STALE", "HOLIDAY":
	default:
		return facts.Observation{}, "source_status_unusable"
	}
	date := v.LastTS.In(kst.Location).Format("20060102")
	if facts.NormalizeBusinessDate(rec.BusinessDate) != date {
		return facts.Observation{}, "date_conflict"
	}
	if v.LastTS.After(rec.TS) || v.LastTS.After(now) {
		return facts.Observation{}, "future_source_timestamp"
	}
	obs := facts.Observation{ProvenanceVersion: facts.CurrentProvenanceVersion, MetricID: "vkospi.value", Unit: "index", BusinessDate: date, ObservedAt: v.LastTS, RetrievedAt: rec.TS, Value: &v.Value, Source: v.Source, SourceField: "vkospi.value", FreshnessStatus: "VALID", Raw: raw}
	if !facts.AcceptedAt(obs, now) {
		return facts.Observation{}, "invalid_source_observation"
	}
	return obs, ""
}

func backfillSnapshots(ctx context.Context, store facts.Store, dir string) (int, error) {
	stats, err := backfillSnapshotsWithSummary(ctx, store, dir)
	return stats.AcceptedAttempts, err
}

func backfillSnapshotsWithSummary(ctx context.Context, store facts.Store, dir string) (backfillSummary, error) {
	stats := backfillSummary{}
	files, err := filepath.Glob(filepath.Join(dir, "market_snapshot.*.json"))
	if err != nil {
		stats.fail("input_path_error", err)
		return stats, stats.Err()
	}
	for _, path := range files {
		stats.Processed++
		if err := ctx.Err(); err != nil {
			stats.fail("context_canceled", err)
			return stats, stats.Err()
		}
		// Archived revisions are preserved as evidence, not reimported as latest.
		if !snapshotFilename(path) {
			stats.skip("archive_or_noncanonical_filename")
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			stats.fail("input_read_error", err)
			continue
		}
		var header struct {
			SchemaVersion int `json:"schema_version"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			stats.fail("invalid_json", fmt.Errorf("invalid snapshot JSON in %s", filepath.Base(path)))
			continue
		}
		if header.SchemaVersion != 2 {
			stats.skip(schemaSkipReason(header.SchemaVersion))
			continue
		}
		var rec snapshot.SnapshotJSON
		if err := json.Unmarshal(raw, &rec); err != nil {
			stats.fail("invalid_json", fmt.Errorf("invalid snapshot JSON in %s", filepath.Base(path)))
			continue
		}
		name := filepath.Base(path)
		fileDate := name[len("market_snapshot.") : len("market_snapshot.")+8]
		if facts.NormalizeBusinessDate(rec.Date) != fileDate {
			stats.skip("date_conflict")
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		if obs, reason := classifiedSnapshotObservation(rec, payload, time.Now()); reason == "" {
			if err := store.Put(ctx, obs); err != nil {
				stats.fail("store_write_error", err)
			} else {
				stats.AcceptedAttempts++
			}
		} else {
			stats.skip(reason)
		}
	}
	return stats, stats.Err()
}

func snapshotFilename(path string) bool {
	name := filepath.Base(path)
	const prefix = "market_snapshot."
	return len(name) == len(prefix)+8+5 && facts.NormalizeBusinessDate(name[len(prefix):len(prefix)+8]) != ""
}

func snapshotObservation(rec snapshot.SnapshotJSON, raw map[string]any, now time.Time) (facts.Observation, bool) {
	obs, reason := classifiedSnapshotObservation(rec, raw, now)
	return obs, reason == ""
}

func classifiedSnapshotObservation(rec snapshot.SnapshotJSON, raw map[string]any, now time.Time) (facts.Observation, string) {
	v := rec.Volatility
	if rec.SchemaVersion != 2 {
		return facts.Observation{}, schemaSkipReason(rec.SchemaVersion)
	}
	if v == nil {
		return facts.Observation{}, "observation_missing"
	}
	if v.Source == "" {
		return facts.Observation{}, "source_missing"
	}
	if v.ObservedAt.IsZero() || rec.Timestamp.IsZero() {
		return facts.Observation{}, "source_timestamp_missing"
	}
	if v.VKOSPI <= 0 || math.IsNaN(v.VKOSPI) || math.IsInf(v.VKOSPI, 0) {
		return facts.Observation{}, "invalid_value"
	}
	if v.Status != snapshot.StatusPreliminary && v.Status != snapshot.StatusFinal && v.Status != snapshot.StatusReconciledProvisional {
		return facts.Observation{}, "source_status_unusable"
	}
	date := facts.NormalizeBusinessDate(v.Date)
	if date == "" || date != facts.NormalizeBusinessDate(rec.Date) || date != v.ObservedAt.In(kst.Location).Format("20060102") {
		return facts.Observation{}, "date_conflict"
	}
	retrieved := rec.CompletedAt
	if retrieved.IsZero() {
		retrieved = rec.Timestamp
	}
	if v.ObservedAt.After(retrieved) || v.ObservedAt.After(now) {
		return facts.Observation{}, "future_source_timestamp"
	}
	obs := facts.Observation{ProvenanceVersion: facts.CurrentProvenanceVersion, MetricID: "vkospi.value", Unit: "index", BusinessDate: date, ObservedAt: v.ObservedAt, RetrievedAt: retrieved, Value: &v.VKOSPI, Source: v.Source, SourceField: "volatility.vkospi", FreshnessStatus: "VALID", Raw: raw}
	if !facts.AcceptedAt(obs, now) {
		return facts.Observation{}, "invalid_source_observation"
	}
	return obs, ""
}
