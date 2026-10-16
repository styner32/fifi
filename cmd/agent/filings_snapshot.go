package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fifi/internal/collector/filings"
	"github.com/fifi/internal/dartfiling/config"
	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/fifi/internal/db"
	"github.com/fifi/internal/external/dart"
	"github.com/fifi/internal/kst"
)

func runFilingsSnapshot(args []string) error {
	fs := flag.NewFlagSet("filings-snapshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dateFlag := fs.String("date", "", "business date YYYY-MM-DD or YYYYMMDD (default: today KST)")
	targetsFlag := fs.String("targets", "", "targets document (default: $"+filings.TargetsFileEnvKey+" or "+filings.DefaultTargetsFile+")")
	noSyncFlag := fs.Bool("no-sync", false, "skip DART fetch/analyze and only render already stored DB disclosures")
	limitFlag := fs.Int("limit", filings.DefaultLimit, "per-company disclosure limit when syncing")
	outDirFlag := fs.String("out-dir", envDefault("SNAPSHOT_OUTPUT_DIR", ".cache/snapshots"), "output directory for market_filings.<YYYYMMDD>.{md,json}")
	if err := fs.Parse(args); err != nil {
		return err
	}

	normalizedDate, err := validateDate(*dateFlag)
	if err != nil {
		return err
	}
	if normalizedDate == "" {
		normalizedDate = kst.Now().Format("20060102")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	targetsPath := filings.ResolveTargetsPath(*targetsFlag)
	mdOut, err := executeFilingsSnapshot(ctx, normalizedDate, *outDirFlag, targetsPath, !*noSyncFlag, *limitFlag)
	if err != nil {
		return err
	}
	fmt.Print(mdOut)
	return nil
}

// runFilingsSnapshotForMarketSnapshot is invoked at the end of `make takesnapshot`
// (`cmd/agent report market-snapshot`). Errors are logged to stderr so the market
// snapshot itself never fails if DB or DART credentials are absent.
func runFilingsSnapshotForMarketSnapshot(date, outDir string) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MARKET_SNAPSHOT_SKIP_FILINGS")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("MARKET_SNAPSHOT_SKIP_FILINGS")), "true") {
		return
	}
	syncDART := !strings.EqualFold(strings.TrimSpace(os.Getenv("MARKET_SNAPSHOT_SKIP_FILINGS_SYNC")), "1") &&
		!strings.EqualFold(strings.TrimSpace(os.Getenv("MARKET_SNAPSHOT_SKIP_FILINGS_SYNC")), "true")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	targetsPath := filings.ResolveTargetsPath("")
	mdOut, err := executeFilingsSnapshot(ctx, date, outDir, targetsPath, syncDART, filings.DefaultLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[snapshot] filings snapshot skipped: %v\n", err)
		return
	}
	fmt.Println()
	fmt.Print(mdOut)
}

func executeFilingsSnapshot(ctx context.Context, date, outDir, targetsPath string, syncDART bool, limit int) (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("load filing config: %w", err)
	}
	dbConn, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		return "", fmt.Errorf("connect database: %w", err)
	}

	targets, err := filings.LoadTargets(targetsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[snapshot] warning: load targets %s: %v\n", targetsPath, err)
	}

	var resolvedTargets []filings.Target
	if strings.TrimSpace(cfg.DartAPIKey) != "" && len(targets) > 0 {
		dartClient := dart.New(cfg.DartAPIKey)
		resolver := filings.NewCorpCodeResolver(dartClient)
		resolved, _, resolveErr := resolver.Resolve(targets)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "[snapshot] warning: resolve corp codes: %v\n", resolveErr)
			resolvedTargets = targets
		} else {
			resolvedTargets = resolved
		}

		if syncDART && strings.TrimSpace(cfg.OpenAIAPIKey) != "" && len(resolvedTargets) > 0 {
			if targetDay, parseErr := time.ParseInLocation("20060102", date, kst.Location); parseErr == nil {
				fmt.Fprintf(os.Stderr, "[snapshot] 당일(%s) 주요 기업 공시 동기화 중 (%d개사)...\n", date, len(resolvedTargets))
				collector := &filings.Collector{
					Fetcher: tasks.NewTaskProcessor(dbConn, cfg),
					Lister:  dartClient,
					Options: filings.Options{
						Limit:     limit,
						StartDate: targetDay,
						EndDate:   targetDay,
					},
					Logf: func(format string, args ...any) {
						fmt.Fprintf(os.Stderr, format+"\n", args...)
					},
				}
				collector.CollectOnce(ctx, resolvedTargets)
			}
		}
	} else {
		resolvedTargets = targets
	}

	snap, err := filings.QuerySnapshotFromDB(ctx, dbConn, date, resolvedTargets)
	if err != nil {
		return "", err
	}
	snap.TargetsFile = targetsPath
	if len(targets) > 0 {
		snap.TargetCount = len(targets)
	}

	mdOut := filings.RenderSnapshot(snap)
	if outDir != "" {
		mdPath, jsonPath, saveErr := filings.SaveSnapshotFiles(snap, outDir)
		if saveErr != nil {
			fmt.Fprintf(os.Stderr, "[snapshot] Filings save failed: %v\n", saveErr)
		} else {
			fmt.Fprintf(os.Stderr, "[snapshot] Filings JSON saved: %s\n", jsonPath)
			fmt.Fprintf(os.Stderr, "[snapshot] Filings MD saved: %s\n", mdPath)
		}
	}
	return mdOut, nil
}

