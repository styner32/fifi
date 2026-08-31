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

// NewPostgresStore creates a new PostgresStore with the provided DSN.
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	gormDB, err := db.InitDB(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to init postgres store: %w", err)
	}
	return &PostgresStore{db: gormDB}, nil
}

// NewPostgresStoreWithDB wraps an existing GORM database handle.
func NewPostgresStoreWithDB(gormDB *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gormDB}
}

// Put inserts one or more observations. Duplicates on (metric_id, observed_at) are ignored.
func (s *PostgresStore) Put(ctx context.Context, obs ...Observation) error {
	if len(obs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&obs).Error
}

// Latest returns the most recent observation for the given metric ID regardless of quality status.
func (s *PostgresStore) Latest(ctx context.Context, metricID string) (*Observation, error) {
	var obs Observation
	err := s.db.WithContext(ctx).
		Where("metric_id = ?", metricID).
		Order("observed_at DESC").
		First(&obs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &obs, nil
}

// LatestValid returns the most recent valid observation for the given metric ID within maxAge.
func (s *PostgresStore) LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*Observation, error) {
	var obs Observation
	query := s.db.WithContext(ctx).
		Where("metric_id = ? AND status IN (?, ?, ?) AND value IS NOT NULL", metricID, "VALID", "FRESH", "PROVISIONAL_LAST_VALID")
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
	return &obs, nil
}

// History queries observation records within a time range.
func (s *PostgresStore) History(ctx context.Context, metricID string, from, to time.Time, limit int) ([]Observation, error) {
	var results []Observation
	query := s.db.WithContext(ctx).Where("metric_id = ?", metricID)
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
	return results, nil
}
