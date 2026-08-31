package facts

import (
	"context"
	"fmt"
	"time"
)

// Resolve attempts to fetch a fresh observation using fetchFunc.
// If fetch succeeds and returns a valid value, it is persisted to the store and returned with FreshnessStatus "VALID".
// If fetch fails or produces no value, it queries the store for the last known valid observation within maxAge.
// If found, it returns the observation with FreshnessStatus "PROVISIONAL_LAST_VALID".
// Otherwise, an observation with FreshnessStatus "UNAVAILABLE" is returned.
func Resolve(
	ctx context.Context,
	store Store,
	metricID string,
	maxAge time.Duration,
	fetch func(context.Context) (Observation, error),
) Observation {
	var fetchErr error
	if fetch != nil {
		obs, err := fetch(ctx)
		if err == nil && obs.Value != nil {
			if obs.FreshnessStatus == "" {
				obs.FreshnessStatus = "VALID"
			}
			if obs.RetrievedAt.IsZero() {
				obs.RetrievedAt = time.Now()
			}
			if obs.MetricID == "" {
				obs.MetricID = metricID
			}

			if store != nil {
				_ = store.Put(ctx, obs)
			}
			return obs
		}
		fetchErr = err
	}

	// Fallback to store
	if store != nil {
		latest, err := store.LatestValid(ctx, metricID, maxAge)
		if err == nil && latest != nil && latest.Value != nil {
			result := *latest
			result.FreshnessStatus = "PROVISIONAL_LAST_VALID"
			result.RetrievedAt = time.Now()
			daysAgo := int(time.Since(latest.ObservedAt).Hours() / 24)
			result.MissingReason = fmt.Sprintf("fallback to last valid observation from %s (%d days ago)", latest.BusinessDate, daysAgo)
			return result
		}
	}

	reason := "observation unavailable and no recent valid observation found"
	if fetchErr != nil {
		reason = fmt.Sprintf("fetch error: %v (no fallback found)", fetchErr)
	}

	return Observation{
		MetricID:        metricID,
		Value:           nil,
		FreshnessStatus: "UNAVAILABLE",
		RetrievedAt:     time.Now(),
		MissingReason:   reason,
	}
}
