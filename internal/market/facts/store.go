package facts

import (
	"context"
	"strings"
	"time"
)

// Store defines the repository operations for storing and querying market observations.
type Store interface {
	Put(ctx context.Context, obs ...Observation) error
	Latest(ctx context.Context, metricID string) (*Observation, error)
	LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*Observation, error)
	History(ctx context.Context, metricID string, from, to time.Time, limit int) ([]Observation, error)
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
