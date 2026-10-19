package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fifi/internal/collector/filings"
	"github.com/fifi/internal/dartfiling/config"
	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/fifi/internal/db"
	"github.com/fifi/internal/external/dart"
	"github.com/fifi/internal/external/openai"
	"github.com/openai/openai-go/v3/shared"
)

const filingsIntervalEnvKey = "FILINGS_COLLECTOR_INTERVAL"

// runFilings dispatches `market-collector filings <once|run> [flags]`.
func runFilings(args []string) error {
	mode := "once"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		mode, args = args[0], args[1:]
	}
	if mode != "once" && mode != "run" {
		return fmt.Errorf("unknown filings mode %q (supported: once, run)", mode)
	}

	fs := flag.NewFlagSet("filings "+mode, flag.ContinueOnError)
	targetsFlag := fs.String("targets", "", "Targets document (default: $"+filings.TargetsFileEnvKey+" or "+filings.DefaultTargetsFile+")")
	limitFlag := fs.Int("limit", filings.DefaultLimit, "Most recent disclosures per company")
	daysFlag := fs.Int("days", filings.DefaultDays, "Lookback window in days")
	dryRunFlag := fs.Bool("dry-run", false, "List disclosures only (no DB writes, no OpenAI calls)")
	intervalFlag := fs.Duration("interval", time.Hour, "Collection interval (run mode; $"+filingsIntervalEnvKey+" overrides)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if strings.TrimSpace(cfg.DartAPIKey) == "" {
		return errors.New("DART_API_KEY is required")
	}
	if !*dryRunFlag && strings.TrimSpace(cfg.OpenAIAPIKey) == "" {
		return errors.New("OPENAI_API_KEY is required (or use --dry-run)")
	}

	dartClient := dart.New(cfg.DartAPIKey)
	collector := &filings.Collector{
		Lister:  dartClient,
		Options: filings.Options{Limit: *limitFlag, Days: *daysFlag, DryRun: *dryRunFlag},
	}
	if !*dryRunFlag {
		dbConn, err := db.InitDB(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		collector.Fetcher = tasks.NewTaskProcessor(dbConn, cfg)
	}
	resolver := filings.NewCorpCodeResolver(dartClient)
	targetsPath := filings.ResolveTargetsPath(*targetsFlag)

	log.Printf("[filings] mode=%s targets=%s limit=%d days=%d dry-run=%v model=%s",
		mode, targetsPath, *limitFlag, *daysFlag, *dryRunFlag, openai.ResolveModel(shared.ResponsesModel(cfg.OpenAIModel)))

	if mode == "once" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		sum, err := filingsTick(ctx, collector, resolver, targetsPath)
		if err != nil {
			return err
		}
		if sum.Failed > 0 {
			return fmt.Errorf("%d of %d companies failed", sum.Failed, len(sum.Companies))
		}
		return nil
	}

	interval := *intervalFlag
	if env := os.Getenv(filingsIntervalEnvKey); env != "" {
		if d, err := time.ParseDuration(env); err == nil && d > 0 {
			interval = d
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Printf("[filings] starting daemon interval=%s", interval)

	runTick := func() {
		if _, err := filingsTick(ctx, collector, resolver, targetsPath); err != nil {
			log.Printf("[filings] tick error: %v", err)
		}
	}
	runTick()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[filings] received shutdown signal, exiting gracefully")
			return nil
		case <-ticker.C:
			runTick()
		}
	}
}

// filingsTick re-reads the targets document every time so edits take effect
// without restarting the daemon.
func filingsTick(ctx context.Context, c *filings.Collector, r *filings.CorpCodeResolver, targetsPath string) (filings.Summary, error) {
	targets, err := filings.LoadTargets(targetsPath)
	if err != nil {
		return filings.Summary{}, err
	}
	if len(targets) == 0 {
		return filings.Summary{}, fmt.Errorf("no targets in %s", targetsPath)
	}
	resolved, unresolved, err := r.Resolve(targets)
	if err != nil {
		return filings.Summary{}, err
	}
	for _, t := range unresolved {
		log.Printf("[filings] skip %s (line %d): stock code not found in DART corpCode.xml; add corp_code manually", t.Label(), t.Line)
	}

	started := time.Now()
	sum := c.CollectOnce(ctx, resolved)
	log.Printf("[filings] tick done in %s: companies=%d failed=%d unresolved=%d stored=%d analyzed=%d tokens=%d",
		time.Since(started).Round(time.Second), len(sum.Companies), sum.Failed, len(unresolved), sum.Stored, sum.Analyzed, sum.UsedTokens)
	return sum, nil
}

