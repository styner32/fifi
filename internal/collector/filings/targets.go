package filings

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// DefaultTargetsFile is the tracked document listing filing collection targets.
const DefaultTargetsFile = "docs/filing_targets.txt"

// TargetsFileEnvKey overrides DefaultTargetsFile.
const TargetsFileEnvKey = "FILING_TARGETS_FILE"

var (
	stockCodePattern = regexp.MustCompile(`^[0-9A-Z]{6}$`) // KRX tickers may be alphanumeric
	corpCodePattern  = regexp.MustCompile(`^[0-9]{8}$`)
)

// Target is one company whose DART filings should be collected.
type Target struct {
	StockCode string // 6-char KRX ticker
	CorpCode  string // 8-digit DART corp code; resolved from StockCode when empty
	Name      string // from the trailing "# comment" or DART corp name
	Line      int    // source line number in the targets document
}

// Label returns a human readable identifier for logs.
func (t Target) Label() string {
	if t.Name != "" {
		return fmt.Sprintf("%s(%s)", t.Name, t.StockCode)
	}
	return t.StockCode
}

// ResolveTargetsPath returns the explicit path, then FILING_TARGETS_FILE, then the default.
func ResolveTargetsPath(explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return p
	}
	if p := strings.TrimSpace(os.Getenv(TargetsFileEnvKey)); p != "" {
		return p
	}
	return DefaultTargetsFile
}

// LoadTargets reads a targets document from disk.
func LoadTargets(path string) ([]Target, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("filing targets: %w", err)
	}
	defer f.Close()
	return ParseTargets(f)
}

// ParseTargets parses the watchlist-style document format:
//
//	<stock_code> [corp_code]  # name
//
// Blank lines and lines starting with '#' are ignored. Duplicate stock codes
// keep the first occurrence. Malformed lines are rejected with their line number
// so a typo in the document is never silently dropped.
func ParseTargets(r io.Reader) ([]Target, error) {
	var targets []Target
	seen := map[string]bool{}
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		name := ""
		if idx := strings.Index(line, "#"); idx >= 0 {
			name = strings.TrimSpace(line[idx+1:])
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 2 {
			return nil, fmt.Errorf("filing targets line %d: expected '<stock_code> [corp_code]', got %q", lineNo, line)
		}
		stock := strings.ToUpper(fields[0])
		if !stockCodePattern.MatchString(stock) {
			return nil, fmt.Errorf("filing targets line %d: invalid stock code %q (want 6 chars)", lineNo, fields[0])
		}
		corp := ""
		if len(fields) == 2 {
			corp = fields[1]
			if !corpCodePattern.MatchString(corp) {
				return nil, fmt.Errorf("filing targets line %d: invalid corp code %q (want 8 digits)", lineNo, corp)
			}
		}
		if seen[stock] {
			continue
		}
		seen[stock] = true
		targets = append(targets, Target{StockCode: stock, CorpCode: corp, Name: name, Line: lineNo})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("filing targets scan: %w", err)
	}
	return targets, nil
}

