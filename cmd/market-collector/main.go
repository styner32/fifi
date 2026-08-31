package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/calendar"
	"github.com/fifi/internal/market/facts"
	"github.com/fifi/internal/parse"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("[market-collector] fatal: %v", err)
	}
}

func run(args []string) error {
	_ = godotenv.Load()

	cmd := "run"
	var cmdArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		cmdArgs = args[1:]
	} else {
		cmdArgs = args
	}

	switch cmd {
	case "run":
		return runDaemon(cmdArgs)
	case "once":
		return runOnce(cmdArgs)
	case "backfill":
		return runBackfill(cmdArgs)
	default:
		return fmt.Errorf("unknown command %q (supported: run, once, backfill)", cmd)
	}
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	intervalFlag := fs.Duration("interval", 3*time.Minute, "Collection interval")
	forceFlag := fs.Bool("force", false, "Force collection regardless of market phase")
	if err := fs.Parse(args); err != nil {
		return err
	}

	interval := *intervalFlag
	if envInterval := os.Getenv("MARKET_COLLECTOR_INTERVAL"); envInterval != "" {
		if d, err := time.ParseDuration(envInterval); err == nil && d > 0 {
			interval = d
		}
	}

	store := facts.NewStore(os.Getenv("DATABASE_URL"))
	client, err := newKISClient()
	if err != nil {
		return fmt.Errorf("failed to init KIS client: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("[market-collector] starting daemon with interval=%s force=%v", interval, *forceFlag)

	// Run initial collection
	collectMetrics(ctx, client, store, *forceFlag)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[market-collector] received shutdown signal, exiting gracefully")
			return nil
		case <-ticker.C:
			collectMetrics(ctx, client, store, *forceFlag)
		}
	}
}

func runOnce(args []string) error {
	fs := flag.NewFlagSet("once", flag.ContinueOnError)
	forceFlag := fs.Bool("force", true, "Force collection regardless of market phase")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := facts.NewStore(os.Getenv("DATABASE_URL"))
	client, err := newKISClient()
	if err != nil {
		return fmt.Errorf("failed to init KIS client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	collectMetrics(ctx, client, store, *forceFlag)
	return nil
}

func collectMetrics(ctx context.Context, client *auth.KIClient, store facts.Store, force bool) {
	nowKST := kst.Now()
	dateStr := nowKST.Format("20060102")
	isHoliday := calendar.IsHoliday("KRX", dateStr)
	phase := calendar.GetMarketPhase("KRX", nowKST, isHoliday)

	if !force && phase != "CONTINUOUS" && phase != "CLOSING_AUCTION" && phase != "POST_CLOSE" {
		log.Printf("[market-collector] market phase is %s on %s, skipping tick", phase, dateStr)
		return
	}

	log.Printf("[market-collector] tick triggered (phase=%s, time=%s)", phase, nowKST.Format("15:04:05"))

	if _, err := client.EnsureAuthToken(ctx); err != nil {
		log.Printf("[market-collector] error ensuring auth token: %v", err)
		return
	}

	stockService := domesticstock.NewService(client)
	var observations []facts.Observation
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 1. VKOSPI
	wg.Add(1)
	go func() {
		defer wg.Done()
		vkCode, err := stockService.ResolveVKOSPICode(ctx, nil)
		if err != nil {
			log.Printf("[market-collector] failed to resolve VKOSPI code: %v", err)
			return
		}
		resp, err := stockService.InquireVKOSPIPrice(ctx, vkCode)
		if err != nil || !resp.IsOK() {
			log.Printf("[market-collector] VKOSPI query failed: %v", err)
			return
		}
		rows := resp.Body["output"]
		var row map[string]any
		switch typed := rows.(type) {
		case []any:
			if len(typed) > 0 {
				if r, ok := typed[0].(map[string]any); ok {
					row = r
				}
			}
		case map[string]any:
			row = typed
		}
		if row != nil {
			if val, ok := parse.Float(row["bstp_nmix_prpr"]); ok && val > 0 {
				mu.Lock()
				observations = append(observations, facts.Observation{
					MetricID:        "vkospi.value",
					BusinessDate:    dateStr,
					ObservedAt:      nowKST,
					RetrievedAt:     time.Now(),
					Value:           &val,
					Source:          "KIS",
					SourceField:     "bstp_nmix_prpr",
					FreshnessStatus: "VALID",
					Raw:             row,
				})
				mu.Unlock()
			}
		}
	}()

	// 2. KOSPI Index
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := stockService.InquireIndexPrice(ctx, "0001")
		if err != nil || !resp.IsOK() {
			log.Printf("[market-collector] KOSPI index query failed: %v", err)
			return
		}
		row, _ := resp.Body["output"].(map[string]any)
		if row != nil {
			if val, ok := parse.Float(row["bstp_nmix_prpr"]); ok && val > 0 {
				mu.Lock()
				observations = append(observations, facts.Observation{
					MetricID:        "kospi.index",
					BusinessDate:    dateStr,
					ObservedAt:      nowKST,
					RetrievedAt:     time.Now(),
					Value:           &val,
					Source:          "KIS",
					SourceField:     "bstp_nmix_prpr",
					FreshnessStatus: "VALID",
					Raw:             row,
				})
				mu.Unlock()
			}
		}
	}()

	// 3. KOSDAQ Index
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := stockService.InquireIndexPrice(ctx, "1001")
		if err != nil || !resp.IsOK() {
			log.Printf("[market-collector] KOSDAQ index query failed: %v", err)
			return
		}
		row, _ := resp.Body["output"].(map[string]any)
		if row != nil {
			if val, ok := parse.Float(row["bstp_nmix_prpr"]); ok && val > 0 {
				mu.Lock()
				observations = append(observations, facts.Observation{
					MetricID:        "kosdaq.index",
					BusinessDate:    dateStr,
					ObservedAt:      nowKST,
					RetrievedAt:     time.Now(),
					Value:           &val,
					Source:          "KIS",
					SourceField:     "bstp_nmix_prpr",
					FreshnessStatus: "VALID",
					Raw:             row,
				})
				mu.Unlock()
			}
		}
	}()

	// 4. Investor Flows
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := stockService.InquireInvestor(ctx, "J", "0001")
		if err != nil || !resp.IsOK() {
			log.Printf("[market-collector] Investor flow query failed: %v", err)
			return
		}
		row, _ := resp.Body["output"].(map[string]any)
		if row != nil {
			mu.Lock()
			if val, ok := parse.Float(row["frgn_ntby_tr_pbmn"]); ok {
				eok := val / 100000000.0
				observations = append(observations, facts.Observation{
					MetricID:        "kospi.flow.foreign_eok",
					BusinessDate:    dateStr,
					ObservedAt:      nowKST,
					RetrievedAt:     time.Now(),
					Value:           &eok,
					Unit:            "억원",
					Source:          "KIS",
					SourceField:     "frgn_ntby_tr_pbmn",
					FreshnessStatus: "VALID",
					Raw:             row,
				})
			}
			if val, ok := parse.Float(row["orgn_ntby_tr_pbmn"]); ok {
				eok := val / 100000000.0
				observations = append(observations, facts.Observation{
					MetricID:        "kospi.flow.organ_eok",
					BusinessDate:    dateStr,
					ObservedAt:      nowKST,
					RetrievedAt:     time.Now(),
					Value:           &eok,
					Unit:            "억원",
					Source:          "KIS",
					SourceField:     "orgn_ntby_tr_pbmn",
					FreshnessStatus: "VALID",
					Raw:             row,
				})
			}
			if val, ok := parse.Float(row["prsn_ntby_tr_pbmn"]); ok {
				eok := val / 100000000.0
				observations = append(observations, facts.Observation{
					MetricID:        "kospi.flow.individual_eok",
					BusinessDate:    dateStr,
					ObservedAt:      nowKST,
					RetrievedAt:     time.Now(),
					Value:           &eok,
					Unit:            "억원",
					Source:          "KIS",
					SourceField:     "prsn_ntby_tr_pbmn",
					FreshnessStatus: "VALID",
					Raw:             row,
				})
			}
			mu.Unlock()
		}
	}()

	wg.Wait()

	if len(observations) > 0 {
		if err := store.Put(ctx, observations...); err != nil {
			log.Printf("[market-collector] error saving %d observations: %v", len(observations), err)
		} else {
			log.Printf("[market-collector] successfully recorded %d observations", len(observations))
		}
	}
}

func runBackfill(args []string) error {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	pulseDir := fs.String("pulse-dir", ".cache/pulse", "Directory with pulse jsonl files")
	snapDir := fs.String("snapshots-dir", ".cache/snapshots", "Directory with snapshot json files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := facts.NewStore(os.Getenv("DATABASE_URL"))
	ctx := context.Background()

	log.Printf("[market-collector] starting backfill from pulse=%s snapshots=%s", *pulseDir, *snapDir)
	pulseCount, err := backfillPulse(ctx, store, *pulseDir)
	if err != nil {
		log.Printf("[market-collector] pulse backfill warning: %v", err)
	}
	snapCount, err := backfillSnapshots(ctx, store, *snapDir)
	if err != nil {
		log.Printf("[market-collector] snapshots backfill warning: %v", err)
	}

	log.Printf("[market-collector] backfill complete! total inserted: pulse=%d snapshots=%d", pulseCount, snapCount)
	return nil
}

func backfillPulse(ctx context.Context, store facts.Store, dir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "pulse_*.jsonl"))
	if err != nil || len(files) == 0 {
		return 0, err
	}

	var totalInserted int
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		var obsList []facts.Observation
		for scanner.Scan() {
			var rec map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				continue
			}
			venue, _ := rec["venue"].(string)
			if venue != "KRX" {
				continue
			}
			tStr, _ := rec["timestamp"].(string)
			t, err := time.Parse(time.RFC3339, tStr)
			if err != nil {
				t = time.Now()
			}
			dateStr := t.In(kst.Location).Format("20060102")

			if vkospi, ok := parse.Float(rec["vkospi"]); ok && vkospi > 0 {
				obsList = append(obsList, facts.Observation{
					MetricID:        "vkospi.value",
					BusinessDate:    dateStr,
					ObservedAt:      t,
					RetrievedAt:     t,
					Value:           &vkospi,
					Source:          "PULSE_BACKFILL",
					FreshnessStatus: "VALID",
					Raw:             rec,
				})
			}
		}
		f.Close()
		if len(obsList) > 0 {
			if err := store.Put(ctx, obsList...); err == nil {
				totalInserted += len(obsList)
			}
		}
	}
	return totalInserted, nil
}

func backfillSnapshots(ctx context.Context, store facts.Store, dir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "market_snapshot.*.json"))
	if err != nil || len(files) == 0 {
		return 0, err
	}

	var totalInserted int
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var snap map[string]any
		if err := json.Unmarshal(raw, &snap); err != nil {
			continue
		}
		dateStr, _ := snap["date"].(string)
		if dateStr == "" {
			continue
		}
		t, err := time.ParseInLocation("20060102", dateStr, kst.Location)
		if err != nil {
			continue
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 15, 30, 0, 0, kst.Location)

		var obsList []facts.Observation
		if vol, ok := snap["volatility"].(map[string]any); ok {
			if vkospi, ok := parse.Float(vol["vkospi"]); ok && vkospi > 0 {
				obsList = append(obsList, facts.Observation{
					MetricID:        "vkospi.value",
					BusinessDate:    dateStr,
					ObservedAt:      t,
					RetrievedAt:     time.Now(),
					Value:           &vkospi,
					Source:          "SNAPSHOT_BACKFILL",
					FreshnessStatus: "VALID",
					Raw:             vol,
				})
			}
		}
		if len(obsList) > 0 {
			if err := store.Put(ctx, obsList...); err == nil {
				totalInserted += len(obsList)
			}
		}
	}
	return totalInserted, nil
}

func newKISClient() (*auth.KIClient, error) {
	appKey, appSecret := strings.TrimSpace(os.Getenv("APP_KEY")), strings.TrimSpace(os.Getenv("APP_SECRET"))
	if appKey == "" || appSecret == "" {
		return nil, fmt.Errorf("APP_KEY and APP_SECRET are required")
	}
	client := auth.NewKIClient(appKey, appSecret, "https://openapi.koreainvestment.com:9443", os.Getenv("USER_AGENT"))
	client.Client = &http.Client{Timeout: 15 * time.Second}
	authTokenFile := os.Getenv("AUTH_TOKEN_FILE")
	if authTokenFile == "" {
		authTokenFile = ".auth_token.json"
	}
	client.SetTokenCachePath(authTokenFile)
	return client, nil
}
