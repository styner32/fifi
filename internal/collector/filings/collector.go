package filings

import (
	"context"
	"log"
	"time"

	"github.com/fifi/internal/dartfiling/tasks"
	"github.com/fifi/internal/external/dart"
	"github.com/fifi/internal/kst"
)

const (
	DefaultLimit = 5 // most recent disclosures per company per tick
	DefaultDays  = 5 // lookback window in calendar days
)

// ReportFetcher stores and analyzes disclosures (implemented by tasks.TaskProcessor).
type ReportFetcher interface {
	FetchAndAnalyzeReports(ctx context.Context, pageInfos ...dart.PageInfo) (tasks.FetchReportsResult, error)
}

// DisclosureLister lists disclosures without side effects (implemented by dart.DartClient).
type DisclosureLister interface {
	GetRecentRawReports(pageInfo ...dart.PageInfo) ([]dart.List, error)
}

// Options controls one collection pass.
type Options struct {
	Limit     int       // per company; <=0 uses DefaultLimit
	Days      int       // lookback; <=0 uses DefaultDays
	StartDate time.Time // optional explicit start date (overrides Now - Days when EndDate is also set)
	EndDate   time.Time // optional explicit end date
	DryRun    bool      // list only: no DB writes, no OpenAI calls
}

// CompanyResult is the outcome for one target.
type CompanyResult struct {
	Target      Target
	Result      tasks.FetchReportsResult
	Disclosures []dart.List // populated in dry-run mode
	Err         error
}

// Summary aggregates one collection pass.
type Summary struct {
	Companies  []CompanyResult
	Failed     int
	Stored     int
	Analyzed   int
	UsedTokens int64
}

// Collector runs filing collection for a list of resolved targets.
type Collector struct {
	Fetcher ReportFetcher
	Lister  DisclosureLister
	Options Options
	Now     func() time.Time
	Logf    func(format string, args ...any)
}

func (c *Collector) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

func (c *Collector) pageInfo(corpCode string) dart.PageInfo {
	limit := c.Options.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if !c.Options.StartDate.IsZero() && !c.Options.EndDate.IsZero() {
		return dart.PageInfo{
			CorpCode:  corpCode,
			Limit:     limit,
			StartDate: c.Options.StartDate,
			EndDate:   c.Options.EndDate,
		}
	}
	now := kst.Now()
	if c.Now != nil {
		now = c.Now().In(kst.Location)
	}
	days := c.Options.Days
	if days <= 0 {
		days = DefaultDays
	}
	return dart.PageInfo{CorpCode: corpCode, Limit: limit, StartDate: now.AddDate(0, 0, -days), EndDate: now}
}

// CollectOnce processes each target sequentially. A failure for one company is
// recorded and does not stop the remaining companies; context cancellation does.
func (c *Collector) CollectOnce(ctx context.Context, targets []Target) Summary {
	var sum Summary
	for _, t := range targets {
		if ctx.Err() != nil {
			c.logf("[filings] context canceled, stopping before %s", t.Label())
			break
		}
		cr := CompanyResult{Target: t}
		pi := c.pageInfo(t.CorpCode)

		if c.Options.DryRun {
			if c.Lister == nil {
				cr.Err = errNoLister
			} else {
				cr.Disclosures, cr.Err = c.Lister.GetRecentRawReports(pi)
			}
			if cr.Err == nil {
				c.logf("[filings] %s corp=%s: %d disclosures (dry-run)", t.Label(), t.CorpCode, len(cr.Disclosures))
				for _, d := range cr.Disclosures {
					c.logf("[filings]   %s %s %s", d.RceptDt, d.RceptNo, d.ReportNm)
				}
			}
		} else {
			if c.Fetcher == nil {
				cr.Err = errNoFetcher
			} else {
				cr.Result, cr.Err = c.Fetcher.FetchAndAnalyzeReports(ctx, pi)
			}
			sum.Stored += cr.Result.Stored
			sum.Analyzed += cr.Result.Analyzed
			sum.UsedTokens += cr.Result.UsedTokens
			if cr.Err == nil {
				c.logf("[filings] %s corp=%s: listed=%d skipped=%d stored=%d analyzed=%d tokens=%d",
					t.Label(), t.CorpCode, cr.Result.Listed, cr.Result.Skipped, cr.Result.Stored, cr.Result.Analyzed, cr.Result.UsedTokens)
			}
		}

		if cr.Err != nil {
			sum.Failed++
			c.logf("[filings] %s corp=%s failed: %v", t.Label(), t.CorpCode, cr.Err)
		}
		sum.Companies = append(sum.Companies, cr)
	}
	return sum
}

type collectorError string

func (e collectorError) Error() string { return string(e) }

const (
	errNoLister  = collectorError("filings collector: dry-run requires a DisclosureLister")
	errNoFetcher = collectorError("filings collector: ReportFetcher is not configured")
)

