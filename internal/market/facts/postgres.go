package facts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fifi/internal/db"
)

// PostgresStore implements Store backed by PostgreSQL via GORM.
type PostgresStore struct {
	db *gorm.DB
}

func (s *PostgresStore) Close() error {
	connection, err := s.db.DB()
	if err != nil {
		return err
	}
	return connection.Close()
}

// NewPostgresStore creates a new PostgresStore with the provided DSN.
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	return NewPostgresStoreWithContext(context.Background(), dsn)
}

func NewPostgresStoreWithContext(ctx context.Context, dsn string) (*PostgresStore, error) {
	gormDB, err := db.InitDBContext(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to init postgres store: %w", err)
	}
	return &PostgresStore{db: gormDB}, nil
}

// NewPostgresStoreWithDB wraps an existing GORM database handle.
func NewPostgresStoreWithDB(gormDB *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gormDB}
}

// Put preserves raw legacy rows; duplicates include provenance_version so a
// corrected current row can coexist with the original evidence.
func (s *PostgresStore) Put(ctx context.Context, obs ...Observation) error {
	if len(obs) == 0 {
		return nil
	}
	for _, o := range obs {
		if o.ProvenanceVersion == CurrentProvenanceVersion && !AcceptedAt(o, time.Now()) {
			return fmt.Errorf("invalid current-provenance observation for %s", o.MetricID)
		}
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&obs).Error
}

func (s *PostgresStore) accepted(ctx context.Context, metricID string) *gorm.DB {
	return s.db.WithContext(ctx).
		Where("metric_id = ? AND provenance_version = ?", metricID, CurrentProvenanceVersion).
		Where("status IN (?, ?, ?) AND value IS NOT NULL", "VALID", "FRESH", "PROVISIONAL_LAST_VALID").
		Where("observed_at > ? AND observed_at <= ?", time.Time{}, time.Now()).
		Where("business_date = (observed_at AT TIME ZONE 'Asia/Seoul')::date").
		Where("value NOT IN ('NaN'::float8, 'Infinity'::float8, '-Infinity'::float8)")
}

// Latest returns the most recent source-validated current-provenance observation.
func (s *PostgresStore) Latest(ctx context.Context, metricID string) (*Observation, error) {
	var obs Observation
	err := s.accepted(ctx, metricID).
		Order("observed_at DESC").
		First(&obs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	obs.BusinessDate = NormalizeBusinessDate(obs.BusinessDate)
	return &obs, nil
}

// LatestValid returns the most recent valid observation for the given metric ID within maxAge.
func (s *PostgresStore) LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*Observation, error) {
	var obs Observation
	query := s.accepted(ctx, metricID)
	if maxAge > 0 {
		cutoff := time.Now().Add(-maxAge)
		query = query.Where("observed_at >= ?", cutoff)
	}
	err := query.Order("observed_at DESC").First(&obs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	obs.BusinessDate = NormalizeBusinessDate(obs.BusinessDate)
	if !AcceptedAt(obs, time.Now()) {
		return nil, nil
	}
	return &obs, nil
}

// History queries observation records within a time range.
func (s *PostgresStore) History(ctx context.Context, metricID string, from, to time.Time, limit int) ([]Observation, error) {
	var results []Observation
	query := s.accepted(ctx, metricID)
	if !from.IsZero() {
		query = query.Where("observed_at >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("observed_at <= ?", to)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("observed_at DESC").Find(&results).Error
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].BusinessDate = NormalizeBusinessDate(results[i].BusinessDate)
	}
	return results, nil
}
