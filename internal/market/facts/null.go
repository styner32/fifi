package facts

import (
	"context"
	"time"
)

// NullStore is a no-op implementation of Store for offline/DB-less operation.
type NullStore struct{}

// NewNullStore creates a new NullStore instance.
func NewNullStore() *NullStore {
	return &NullStore{}
}

func (s *NullStore) Put(ctx context.Context, obs ...Observation) error {
	return nil
}

func (s *NullStore) Latest(ctx context.Context, metricID string) (*Observation, error) {
	return nil, nil
}

func (s *NullStore) LatestValid(ctx context.Context, metricID string, maxAge time.Duration) (*Observation, error) {
	return nil, nil
}

func (s *NullStore) History(ctx context.Context, metricID string, from, to time.Time, limit int) ([]Observation, error) {
	return nil, nil
}
