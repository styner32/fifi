package filings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fifi/internal/dartfiling/models"
	"github.com/fifi/internal/external/openai"
	"github.com/fifi/internal/kst"
	"gorm.io/gorm"
)

// SnapshotEntry holds one disclosure and its structured OpenAI analysis for a given business date.
type SnapshotEntry struct {
	ReceiptNumber      string                     `json:"receipt_number"`
	ReceiptDate        string                     `json:"receipt_date"`
	CorpCode           string                     `json:"corp_code"`
	StockCode          string                     `json:"stock_code,omitempty"`
	CompanyName        string                     `json:"company_name"`
	ReportName         string                     `json:"report_name"`
	ReportType         string                     `json:"report_type,omitempty"`
	Summary            string                     `json:"summary,omitempty"`
	PrimaryCause       string                     `json:"primary_cause,omitempty"`
	Details            string                     `json:"details,omitempty"`
	FinancialSpecifics []openai.FinancialSpecific `json:"financial_specifics,omitempty"`
	ConditionsTerms    []openai.ConditionsTerm    `json:"conditions_terms,omitempty"`
	TimePeriods        []openai.TimePeriod        `json:"time_periods,omitempty"`
	EntityAttributes   []openai.EntityAttribute   `json:"entity_attributes,omitempty"`
	RelatedCompanies   []string                   `json:"related_companies,omitempty"`
	UsedTokens         int64                      `json:"used_tokens,omitempty"`
	Analyzed           bool                       `json:"analyzed"`
}

// Snapshot aggregates all disclosures for a single business date (YYYYMMDD).
type Snapshot struct {
	Date        string          `json:"date"`
	GeneratedAt time.Time       `json:"generated_at"`
	TargetsFile string          `json:"targets_file,omitempty"`
	TargetCount int             `json:"target_count"`
	FilingCount int             `json:"filing_count"`
	Entries     []SnapshotEntry `json:"entries"`
}

// ParseAnalysisJSON extracts structured fields from the JSON stored in analyses.analysis.
func ParseAnalysisJSON(raw json.RawMessage) SnapshotEntry {
	var entry SnapshotEntry
	if len(raw) == 0 {
		return entry
	}
	var def openai.DefaultReport
	if err := json.Unmarshal(raw, &def); err == nil && (def.Summary != "" || def.PrimaryCause != "" || def.CompanyName != "" || len(def.DataExtraction.FinancialSpecifics) > 0) {
		entry.CompanyName = strings.TrimSpace(def.CompanyName)
		entry.ReportType = strings.TrimSpace(def.Type)
		entry.Summary = strings.TrimSpace(def.Summary)
		entry.PrimaryCause = strings.TrimSpace(def.PrimaryCause)
		entry.Details = strings.TrimSpace(def.Details)
		entry.FinancialSpecifics = def.DataExtraction.FinancialSpecifics
		entry.ConditionsTerms = def.DataExtraction.ConditionsTerms
		entry.TimePeriods = def.DataExtraction.TimePeriod
		entry.EntityAttributes = def.DataExtraction.EntityAttributes
		entry.RelatedCompanies = def.RelatedCompanies
		return entry
	}

	// Fallback for periodic reports (사업/반기/분기보고서)
	var periodic openai.Report
	if err := json.Unmarshal(raw, &periodic); err == nil && periodic.CompanyName != "" {
		entry.CompanyName = strings.TrimSpace(periodic.CompanyName)
		entry.ReportType = "정기보고서"
		if periodic.PeriodStart != "" || periodic.PeriodEnd != "" {
			entry.Summary = fmt.Sprintf("대상 기간: %s ~ %s (제출일: %s)", periodic.PeriodStart, periodic.PeriodEnd, periodic.SubmissionDate)
		}
	}
	return entry
}

// QuerySnapshotFromDB queries raw_reports and analyses for a specific YYYYMMDD receipt date.
// If targets are provided, their StockCode and Name are used to enrich matching corp_codes.
func QuerySnapshotFromDB(ctx context.Context, dbConn *gorm.DB, date string, targets []Target) (*Snapshot, error) {
	cleanDate := strings.ReplaceAll(strings.TrimSpace(date), "-", "")
	if len(cleanDate) != 8 {
		return nil, fmt.Errorf("invalid snapshot date %q (want YYYYMMDD)", date)
	}

	byCorp := make(map[string]Target, len(targets))
	for _, t := range targets {
		if t.CorpCode != "" {
			byCorp[t.CorpCode] = t
		}
	}

	var rawReports []models.RawReport
	if err := dbConn.WithContext(ctx).
		Where("receipt_number LIKE ?", cleanDate+"%").
		Order("receipt_number DESC, id DESC").
		Find(&rawReports).Error; err != nil {
		return nil, fmt.Errorf("query raw_reports for %s: %w", cleanDate, err)
	}

	snap := &Snapshot{
		Date:        cleanDate,
		GeneratedAt: kst.Now(),
		TargetCount: len(targets),
		Entries:     make([]SnapshotEntry, 0, len(rawReports)),
	}

	if len(rawReports) == 0 {
		return snap, nil
	}

	reportIDs := make([]uint, 0, len(rawReports))
	for _, r := range rawReports {
		reportIDs = append(reportIDs, r.ID)
	}

	var analyses []models.Analysis
	if err := dbConn.WithContext(ctx).
		Where("raw_report_id IN ?", reportIDs).
		Order("id DESC").
		Find(&analyses).Error; err != nil {
		return nil, fmt.Errorf("query analyses for %s: %w", cleanDate, err)
	}

	analysisByReportID := make(map[uint]models.Analysis, len(analyses))
	for _, a := range analyses {
		if _, exists := analysisByReportID[a.RawReportID]; !exists {
			analysisByReportID[a.RawReportID] = a
		}
	}

	for _, r := range rawReports {
		var entry SnapshotEntry
		if a, ok := analysisByReportID[r.ID]; ok {
			entry = ParseAnalysisJSON(a.Analysis)
			entry.Analyzed = true
			entry.UsedTokens = a.UsedTokens
		}
		entry.ReceiptNumber = strings.TrimSpace(r.ReceiptNumber)
		if len(entry.ReceiptNumber) >= 8 {
			entry.ReceiptDate = entry.ReceiptNumber[:8]
		} else {
			entry.ReceiptDate = cleanDate
		}
		entry.CorpCode = strings.TrimSpace(r.CorpCode)
		entry.ReportName = strings.TrimSpace(r.ReportName)

		if t, ok := byCorp[entry.CorpCode]; ok {
			entry.StockCode = t.StockCode
			if t.Name != "" {
				entry.CompanyName = t.Name
			}
		}
		if entry.CompanyName == "" {
			var comp models.Company
			if err := dbConn.WithContext(ctx).Where("corp_code = ?", entry.CorpCode).First(&comp).Error; err == nil && comp.CorpName != "" {
				entry.CompanyName = strings.TrimSpace(comp.CorpName)
			} else {
				entry.CompanyName = entry.CorpCode
			}
		}
		snap.Entries = append(snap.Entries, entry)
	}
	snap.FilingCount = len(snap.Entries)
	return snap, nil
}

// RenderSnapshot formats a Snapshot as a human-readable Markdown report.
func RenderSnapshot(snap *Snapshot) string {
	if snap == nil {
		return ""
	}
	var b strings.Builder
	genAt := snap.GeneratedAt.In(kst.Location)
	formattedDate := snap.Date
	if len(snap.Date) == 8 {
		formattedDate = fmt.Sprintf("%s-%s-%s", snap.Date[:4], snap.Date[4:6], snap.Date[6:8])
	}

	fmt.Fprintf(&b, "# DART 주요 기업 공시 스냅샷 — %s\n", snap.Date)
	fmt.Fprintf(&b, "- 기준일 (접수일): %s\n", formattedDate)
	fmt.Fprintf(&b, "- 생성 시각: %s KST\n", genAt.Format("2006-01-02 15:04:05"))
	if snap.TargetsFile != "" {
		fmt.Fprintf(&b, "- 감시 대상 기업: %d개사 (`%s`) · 당일 공시: %d건\n", snap.TargetCount, snap.TargetsFile, snap.FilingCount)
	} else {
		fmt.Fprintf(&b, "- 당일 공시: %d건\n", snap.FilingCount)
	}

	b.WriteString("\n## 1. 당일 공시 요약\n")
	if len(snap.Entries) == 0 {
		fmt.Fprintf(&b, "- %s 당일 접수된 주요 기업 공시가 없습니다.\n", formattedDate)
		return b.String()
	}

	b.WriteString("| 접수번호 | 기업 | 보고서명 | 핵심 요약 |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, e := range snap.Entries {
		companyLabel := e.CompanyName
		if e.StockCode != "" {
			companyLabel = fmt.Sprintf("%s (%s)", e.CompanyName, e.StockCode)
		}
		summary := e.Summary
		if summary == "" {
			if e.Analyzed {
				summary = "(요약 없음)"
			} else {
				summary = "(미분석 원문)"
			}
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n",
			escapeMDCell(e.ReceiptNumber),
			escapeMDCell(companyLabel),
			escapeMDCell(e.ReportName),
			escapeMDCell(summary),
		)
	}

	b.WriteString("\n## 2. 공시별 상세 분석\n")
	for i, e := range snap.Entries {
		companyLabel := e.CompanyName
		if e.StockCode != "" {
			companyLabel = fmt.Sprintf("%s (%s)", e.CompanyName, e.StockCode)
		}
		fmt.Fprintf(&b, "\n### [%d] %s — %s\n", i+1, companyLabel, e.ReportName)
		fmt.Fprintf(&b, "- **접수번호**: `%s` (`corp_code`: `%s`)\n", e.ReceiptNumber, e.CorpCode)
		if e.ReportType != "" && e.ReportType != e.ReportName {
			fmt.Fprintf(&b, "- **유형**: %s\n", e.ReportType)
		}
		if e.Summary != "" {
			fmt.Fprintf(&b, "- **핵심 요약**: %s\n", e.Summary)
		}
		if e.PrimaryCause != "" {
			fmt.Fprintf(&b, "- **주요 배경/사유**: %s\n", e.PrimaryCause)
		}
		if e.Details != "" && e.Details != "주주 및 임원의 변동에 관한 내용은 없다." {
			fmt.Fprintf(&b, "- **세부 사항**: %s\n", e.Details)
		}
		if len(e.FinancialSpecifics) > 0 {
			b.WriteString("- **주요 수치 및 재무 항목**:\n")
			for _, fs := range e.FinancialSpecifics {
				valUnit := strings.TrimSpace(fs.Value + " " + fs.Unit)
				if fs.Details != "" {
					fmt.Fprintf(&b, "  - **%s**: %s — %s\n", fs.Item, valUnit, fs.Details)
				} else {
					fmt.Fprintf(&b, "  - **%s**: %s\n", fs.Item, valUnit)
				}
			}
		}
		if len(e.ConditionsTerms) > 0 {
			b.WriteString("- **주요 조건 및 참고사항**:\n")
			for _, ct := range e.ConditionsTerms {
				fmt.Fprintf(&b, "  - **%s**: %s\n", ct.TermName, ct.Content)
			}
		}
		if len(e.TimePeriods) > 0 {
			var validPeriods []openai.TimePeriod
			for _, tp := range e.TimePeriods {
				if strings.TrimSpace(tp.Date) != "" {
					validPeriods = append(validPeriods, tp)
				}
			}
			if len(validPeriods) > 0 {
				b.WriteString("- **관련 일정 및 기간**:\n")
				for _, tp := range validPeriods {
					if tp.Note != "" {
						fmt.Fprintf(&b, "  - **%s**: %s (%s)\n", tp.Label, tp.Date, tp.Note)
					} else {
						fmt.Fprintf(&b, "  - **%s**: %s\n", tp.Label, tp.Date)
					}
				}
			}
		}
	}

	return b.String()
}

// SaveSnapshotFiles writes market_filings.<YYYYMMDD>.json and market_filings.<YYYYMMDD>.md
// under outDir, archiving any existing files with a .previous.<nano> suffix.
func SaveSnapshotFiles(snap *Snapshot, outDir string) (mdPath string, jsonPath string, err error) {
	if snap == nil {
		return "", "", fmt.Errorf("nil filings snapshot")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create output dir: %w", err)
	}

	jsonPath = filepath.Join(outDir, fmt.Sprintf("market_filings.%s.json", snap.Date))
	if err := archiveIfExists(jsonPath); err != nil {
		return "", "", err
	}
	rawJSON, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal filings snapshot JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, rawJSON, 0o644); err != nil {
		return "", "", fmt.Errorf("write filings snapshot JSON: %w", err)
	}

	mdPath = filepath.Join(outDir, fmt.Sprintf("market_filings.%s.md", snap.Date))
	if err := archiveIfExists(mdPath); err != nil {
		return "", jsonPath, err
	}
	mdContent := RenderSnapshot(snap)
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		return "", jsonPath, fmt.Errorf("write filings snapshot MD: %w", err)
	}

	return mdPath, jsonPath, nil
}

func archiveIfExists(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	archive := fmt.Sprintf("%s.previous.%d", path, time.Now().UnixNano())
	return os.WriteFile(archive, raw, 0o600)
}

func escapeMDCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}

