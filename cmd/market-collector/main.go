package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/market/facts"
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
	case "filings":
		return runFilings(cmdArgs)
	default:
		return fmt.Errorf("unknown command %q (supported: run, once, backfill, filings)", cmd)
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

	store, err := newCollectorStore(context.Background())
	if err != nil {
		return err
	}
	defer store.Close()
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

	store, err := newCollectorStore(context.Background())
	if err != nil {
		return err
	}
	defer store.Close()
	client, err := newKISClient()
	if err != nil {
		return fmt.Errorf("failed to init KIS client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	collectMetrics(ctx, client, store, *forceFlag)
	return nil
}

func runBackfill(args []string) error {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	pulseDir := fs.String("pulse-dir", ".cache/pulse", "Directory with pulse jsonl files")
	snapDir := fs.String("snapshots-dir", ".cache/snapshots", "Directory with snapshot json files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	store, err := newCollectorStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	log.Printf("[market-collector] starting backfill from pulse=%s snapshots=%s", *pulseDir, *snapDir)
	pulseStats, pulseErr := backfillPulseWithSummary(ctx, store, *pulseDir)
	writeBackfillSummary(os.Stderr, "pulse", pulseStats)
	if pulseErr != nil {
		log.Printf("[market-collector] pulse backfill warning: %v", pulseErr)
	}
	snapshotStats, snapshotErr := backfillSnapshotsWithSummary(ctx, store, *snapDir)
	writeBackfillSummary(os.Stderr, "snapshot", snapshotStats)
	if snapshotErr != nil {
		log.Printf("[market-collector] snapshots backfill warning: %v", snapshotErr)
	}

	return errors.Join(pulseErr, snapshotErr)
}

func newCollectorStore(ctx context.Context) (*facts.PostgresStore, error) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required for persistent market collection")
	}
	return facts.NewPostgresStoreWithContext(ctx, dsn)
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
