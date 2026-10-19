package facts

import (
	"math"
	"strings"
	"time"

	"github.com/fifi/internal/kst"
)

const CurrentProvenanceVersion = 1

// Observation represents a normalized data measurement with full metadata.
type Observation struct {
	ProvenanceVersion int            `json:"provenance_version" gorm:"column:provenance_version;not null;default:0"`
	ID                int64          `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	MetricID          string         `json:"metric_id" gorm:"column:metric_id;not null"`
	BusinessDate      string         `json:"business_date" gorm:"column:business_date;type:date;not null"`
	ObservedAt        time.Time      `json:"observed_at" gorm:"column:observed_at;type:timestamptz;not null"`
	RetrievedAt       time.Time      `json:"retrieved_at" gorm:"column:retrieved_at;type:timestamptz;not null;default:now()"`
	Value             *float64       `json:"value" gorm:"column:value"`
	Unit              string         `json:"unit,omitempty" gorm:"column:unit"`
	Source            string         `json:"source,omitempty" gorm:"column:source"`
	SourceField       string         `json:"source_field,omitempty" gorm:"column:source_field"`
	FreshnessStatus   string         `json:"status" gorm:"column:status;not null"`
	MissingReason     string         `json:"missing_reason,omitempty" gorm:"column:missing_reason"`
	Raw               map[string]any `json:"raw,omitempty" gorm:"column:raw;type:jsonb;serializer:json"`
}

// AcceptedAt excludes legacy rows and values without a verified source timestamp.
// RetrievedAt is deliberately not a substitute for ObservedAt.
func AcceptedAt(o Observation, now time.Time) bool {
	date := NormalizeBusinessDate(o.BusinessDate)
	if o.ProvenanceVersion != CurrentProvenanceVersion || o.Value == nil || math.IsNaN(*o.Value) || math.IsInf(*o.Value, 0) || o.ObservedAt.IsZero() || o.ObservedAt.After(now) || date == "" || date > now.In(kst.Location).Format("20060102") {
		return false
	}
	// Current facts are domestic cash-market measurements. Night-session
	// reference dates need a separate explicit contract before entering this store.
	if date != o.ObservedAt.In(kst.Location).Format("20060102") {
		return false
	}
	switch o.FreshnessStatus {
	case "VALID", "FRESH", "PROVISIONAL_LAST_VALID":
		return true
	default:
		return false
	}
}

func NormalizeBusinessDate(value string) string {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"20060102", "2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format("20060102")
		}
	}
	return ""
}

// TableName returns the table name for GORM.
func (Observation) TableName() string {
	return "market_observations"
}
