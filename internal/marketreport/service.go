// Package marketreport runs the same market collectors for CLI and MCP callers.
// Collection and persistence share a file lock, including token/master caches.
package marketreport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/kst"
)

const (
	Premarket     = "premarket"
	IntradayPulse = "intraday-pulse"
	DailySnapshot = "market-snapshot"
)

type Artifact struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Envelope keeps collector JSON intact; absent measurements remain null.
type Envelope struct {
	SchemaVersion int            `json:"schema_version"`
	ReportType    string         `json:"report_type"`
	Report        map[string]any `json:"report"`
	Artifacts     []Artifact     `json:"artifacts"`
	Warnings      []string       `json:"warnings"`
}

type Result struct {
	Envelope  Envelope
	Markdown  string
	Premarket *premarket.PremarketReport
	Pulse     *pulse.Pulse
	Snapshot  *snapshot.Snapshot
}

type Config struct {
	PremarketDir string
	PulseDir     string
	SnapshotDir  string
	LockFile     string
	Timeout      time.Duration
	Clock        func() time.Time
}

func ConfigFromEnv() Config {
	return Config{PremarketDir: env("PREMARKET_STORE_DIR", ".cache/premarket"), PulseDir: env("PULSE_OUTPUT_DIR", ".cache/pulse"), SnapshotDir: env("SNAPSHOT_OUTPUT_DIR", ".cache/snapshots"), LockFile: env("MARKET_REPORT_LOCK_FILE", ".cache/market-report.lock"), Timeout: 2 * time.Minute, Clock: kst.Now}
}

type Collectors struct {
	Premarket func(context.Context, premarket.Options) (*premarket.PremarketReport, error)
	Pulse     func(context.Context, pulse.Options) (*pulse.Pulse, error)
	Snapshot  func(context.Context, snapshot.Options) (*snapshot.Snapshot, error)
}

type Service struct {
	config     Config
	collectors Collectors
}

func New(config Config) *Service {
	s := NewWithCollectors(config, Collectors{})
	s.collectors = liveCollectors(s.config)
	return s
}

func NewWithCollectors(config Config, collectors Collectors) *Service {
	defaults := ConfigFromEnv()
	if config.PremarketDir == "" {
		config.PremarketDir = defaults.PremarketDir
	}
	if config.PulseDir == "" {
		config.PulseDir = defaults.PulseDir
	}
	if config.SnapshotDir == "" {
		config.SnapshotDir = defaults.SnapshotDir
	}
	if config.LockFile == "" {
		config.LockFile = defaults.LockFile
	}
	if config.Timeout <= 0 || config.Timeout > 2*time.Minute {
		config.Timeout = 2 * time.Minute
	}
	if config.Clock == nil {
		config.Clock = kst.Now
	}
	return &Service{config: config, collectors: collectors}
}

func (s *Service) RunPremarket(ctx context.Context, opts premarket.Options) (result *Result, err error) {
	result = emptyResult(Premarket)
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	unlock, err := acquireLock(ctx, s.config.LockFile)
	if err != nil {
		return failed(result, err)
	}
	defer unlock()
	if opts.StoreDir == "" {
		opts.StoreDir = s.config.PremarketDir
	}
	if s.collectors.Premarket == nil {
		return failed(result, errors.New("premarket collector unavailable"))
	}
	r, err := s.collectors.Premarket(ctx, opts)
	if r == nil {
		return failed(result, firstError(err, errors.New("premarket collector returned no report")))
	}
	result.Premarket = r
	if e := result.setReport(r); e != nil {
		return failed(result, e)
	}
	result.Markdown = premarket.Render(r)
	result.addWarnings(r.Errors)
	if !opts.NoSave && r.Errors["date"] == "" && r.Errors["store_save"] == "" && r.Errors["store_load"] == "" {
		if path := filepath.Join(opts.StoreDir, "store.v2.json"); fileExists(path) {
			result.addArtifact("history", path)
		}
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return failed(result, err)
	}
	available := false
	for _, values := range []map[string]premarket.Observation{r.Inputs, r.DomesticInputs} {
		for _, v := range values {
			if v.Value != nil && (v.Status == "VALID" || v.Status == "LAGGED_CONTEXT_ONLY") {
				available = true
			}
		}
	}
	if !available {
		return failed(result, errors.New("no usable market observations; inspect report errors and missing-data statuses"))
	}
	if !opts.NoSave {
		result.persistPair(opts.StoreDir, "premarket."+r.Date, r)
	}
	return result, nil
}

func (s *Service) RunPulse(ctx context.Context, opts pulse.Options) (result *Result, err error) {
	result = emptyResult(IntradayPulse)
	saveCurrentReport := opts.CurrentOnly && !opts.NoSave
	if opts.CurrentOnly {
		// Current-only calls may save standalone reports, never observation rows.
		opts.NoHistory, opts.NoSave = true, true
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	unlock, err := acquireLock(ctx, s.config.LockFile)
	if err != nil {
		return failed(result, err)
	}
	defer unlock()
	if opts.StoreDir == "" {
		opts.StoreDir = s.config.PulseDir
	}
	if s.collectors.Pulse == nil {
		return failed(result, errors.New("intraday collector unavailable"))
	}
	r, err := s.collectors.Pulse(ctx, opts)
	if r == nil {
		return failed(result, firstError(err, errors.New("intraday collector returned no report")))
	}
	if opts.CurrentOnly {
		r.SetCurrentOnly()
	}
	result.Pulse = r
	if e := result.setReport(pulse.PulseToMap(r)); e != nil {
		return failed(result, e)
	}
	result.Markdown = pulse.Render(r)
	result.addWarnings(r.Errors)
	if r.Saved {
		result.addArtifact("history", pulse.StorePath(opts, r.Date))
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return failed(result, err)
	}
	available := r.KOSPI.Index.OK || r.KOSDAQ.Index.OK || r.KOSPI.Flow.OK || r.KOSDAQ.Flow.OK || r.VKOSPI.OK || r.USDKRW.OK || r.KOSPI200Future.OK || r.KOSDAQ150Future.OK
	for _, v := range r.Macro {
		available = available || v.OK
	}
	if !available {
		return failed(result, errors.New("no usable market observations; inspect report errors and missing-data statuses"))
	}
	if saveCurrentReport {
		name := "pulse_current." + r.Now.In(kst.Location).Format("20060102T150405.000000000")
		result.persistPair(opts.StoreDir, name, pulse.PulseToMap(r))
	} else if !opts.NoSave {
		path := pulse.MDPath(opts, r.Date)
		if e := writeAtomic(path, []byte(result.Markdown)); e != nil {
			result.Envelope.Warnings = append(result.Envelope.Warnings, "Markdown save failed: "+e.Error())
		} else {
			result.addArtifact("markdown", path)
		}
	}
	return result, nil
}

// RunSnapshot saves market artifacts only. Filing sync and LLM processing remain
// explicit CLI orchestration and can never be invoked through this service.
func (s *Service) RunSnapshot(ctx context.Context, opts snapshot.Options, save bool) (result *Result, err error) {
	result = emptyResult(DailySnapshot)
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	unlock, err := acquireLock(ctx, s.config.LockFile)
	if err != nil {
		return failed(result, err)
	}
	defer unlock()
	inputDate := strings.TrimSpace(opts.Date)
	if inputDate == "" {
		inputDate = s.config.Clock().In(kst.Location).Format("20060102")
	}
	layout := "20060102"
	if len(inputDate) == 10 {
		layout = "2006-01-02"
	}
	parsed, e := time.ParseInLocation(layout, inputDate, kst.Location)
	date := parsed.Format("20060102")
	if e != nil || parsed.Format(layout) != inputDate || date > s.config.Clock().In(kst.Location).Format("20060102") {
		return failed(result, errors.New("date must be a valid non-future KST date (YYYYMMDD or YYYY-MM-DD)"))
	}
	opts.Date = date
	if opts.OutputDir == "" {
		opts.OutputDir = s.config.SnapshotDir
	}
	if opts.PulseStoreDir == "" {
		opts.PulseStoreDir = s.config.PulseDir
	}
	if s.collectors.Snapshot == nil {
		return failed(result, errors.New("snapshot collector unavailable"))
	}
	r, err := s.collectors.Snapshot(ctx, opts)
	if r == nil {
		return failed(result, firstError(err, errors.New("snapshot collector returned no report")))
	}
	result.Snapshot = r
	if e := result.setReport(r.ToJSON()); e != nil {
		return failed(result, e)
	}
	previous, e := snapshot.LoadPreviousSnapshot(opts.OutputDir, date)
	if e != nil {
		result.Envelope.Warnings = append(result.Envelope.Warnings, "previous snapshot unavailable: "+e.Error())
	}
	result.Markdown = snapshot.Render(r, previous)
	errorsMap := map[string]string{}
	for key, e := range r.Errors {
		if e != nil {
			errorsMap[key] = e.Error()
		}
	}
	result.addWarnings(errorsMap)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return failed(result, err)
	}
	available := r.Price != nil || r.Flow != nil || r.Credit != nil || (r.Volatility != nil && (r.Volatility.VKOSPI > 0 || r.Volatility.VIX > 0)) || (r.Global != nil && len(r.Global.Quotes) > 0)
	if !available {
		return failed(result, errors.New("no usable market observations; inspect report errors and missing-data statuses"))
	}
	if save {
		result.persistPair(opts.OutputDir, "market_snapshot."+date, r.ToJSON())
	}
	return result, nil
}

func emptyResult(kind string) *Result {
	return &Result{Envelope: Envelope{SchemaVersion: 1, ReportType: kind, Report: map[string]any{}, Artifacts: []Artifact{}, Warnings: []string{}}}
}
func (r *Result) setReport(report any) error {
	raw, e := json.Marshal(report)
	if e != nil {
		return e
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(&r.Envelope.Report)
}
func (r *Result) addWarnings(items map[string]string) {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r.Envelope.Warnings = append(r.Envelope.Warnings, key+": "+items[key])
	}
}
func (r *Result) addArtifact(kind, path string) {
	if absolute, e := filepath.Abs(path); e == nil {
		path = absolute
	}
	r.Envelope.Artifacts = append(r.Envelope.Artifacts, Artifact{Kind: kind, Path: path})
}
func failed(r *Result, e error) (*Result, error) {
	r.Envelope.Warnings = append(r.Envelope.Warnings, e.Error())
	return r, e
}
func firstError(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (r *Result) persistPair(dir, base string, data any) {
	raw, e := json.MarshalIndent(data, "", "  ")
	if e != nil {
		r.Envelope.Warnings = append(r.Envelope.Warnings, "JSON serialization failed: "+e.Error())
		return
	}
	for _, file := range []struct {
		kind, ext string
		data      []byte
	}{{"json", ".json", raw}, {"markdown", ".md", []byte(r.Markdown)}} {
		path := filepath.Join(dir, base+file.ext)
		if e := archiveAndWrite(path, file.data); e != nil {
			r.Envelope.Warnings = append(r.Envelope.Warnings, fmt.Sprintf("%s save failed: %v", file.kind, e))
		} else {
			r.addArtifact(file.kind, path)
		}
	}
}
