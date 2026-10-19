package facts

import (
	"context"
	"strings"
	"time"
)

// Store preserves raw writes. All reads expose accepted current provenance only;
// legacy rows remain in PostgreSQL for explicit evidence audits.
type Store interface {
	Put(ctx context.Context, obs ...Observation) error
	Latest(ctx context.Context, metricID string) (*Observation, error)
	LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*Observation, error)
	History(ctx context.Context, metricID string, from, to time.Time, limit int) ([]Observation, error)
}

// LatestValidReader is the read-only dependency used by report collectors.
type LatestValidReader interface {
	LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*Observation, error)
}

// NewStoreWithContext reports connection failures and bounds connection setup.
func NewStoreWithContext(ctx context.Context, databaseURL string) (Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return NewNullStore(), nil
	}
	return NewPostgresStoreWithContext(ctx, databaseURL)
}

// NewStore initializes a PostgresStore if databaseURL is provided and reachable, otherwise falls back to NullStore.
func NewStore(databaseURL string) Store {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return NewNullStore()
	}

	store, err := NewPostgresStore(databaseURL)
	if err != nil {
		return NewNullStore()
	}
	return store
}
