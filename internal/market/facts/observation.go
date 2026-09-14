package facts

import "time"

// Observation represents a normalized data measurement with full metadata.
type Observation struct {
	ID              int64          `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	MetricID        string         `json:"metric_id" gorm:"column:metric_id;not null"`
	BusinessDate    string         `json:"business_date" gorm:"column:business_date;type:date;not null"`
	ObservedAt      time.Time      `json:"observed_at" gorm:"column:observed_at;type:timestamptz;not null"`
	RetrievedAt     time.Time      `json:"retrieved_at" gorm:"column:retrieved_at;type:timestamptz;not null;default:now()"`
	Value           *float64       `json:"value" gorm:"column:value"`
	Unit            string         `json:"unit,omitempty" gorm:"column:unit"`
	Source          string         `json:"source,omitempty" gorm:"column:source"`
	SourceField     string         `json:"source_field,omitempty" gorm:"column:source_field"`
	FreshnessStatus string         `json:"status" gorm:"column:status;not null"`
	MissingReason   string         `json:"missing_reason,omitempty" gorm:"column:missing_reason"`
	Raw             map[string]any `json:"raw,omitempty" gorm:"column:raw;type:jsonb;serializer:json"`
}

// TableName returns the table name for GORM.
func (Observation) TableName() string {
	return "market_observations"
}
