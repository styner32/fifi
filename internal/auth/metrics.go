package auth

import (
	"net/http"
	"time"
)

type httpCallMetrics struct {
	callCount     int
	successCount  int
	errorCount    int
	totalDuration time.Duration
	startedAt     time.Time
	lastCallAt    time.Time
}

type HTTPMetricsSnapshot struct {
	CallCount     int
	SuccessCount  int
	ErrorCount    int
	TotalDuration time.Duration
	AverageTime   time.Duration
	StartedAt     time.Time
	LastCallAt    time.Time
	Elapsed       time.Duration
	RPM           float64
}

func (client *KIClient) Do(req *http.Request) (*http.Response, error) {
	startedAt := time.Now()
	resp, err := client.Client.Do(req)
	client.recordHTTPMetrics(startedAt, time.Now(), err)
	return resp, err
}

func (client *KIClient) recordHTTPMetrics(startedAt time.Time, endedAt time.Time, err error) {
	client.metricsMu.Lock()
	defer client.metricsMu.Unlock()

	if client.metrics.startedAt.IsZero() || startedAt.Before(client.metrics.startedAt) {
		client.metrics.startedAt = startedAt
	}
	if endedAt.After(client.metrics.lastCallAt) {
		client.metrics.lastCallAt = endedAt
	}

	client.metrics.callCount++
	client.metrics.totalDuration += endedAt.Sub(startedAt)

	if err != nil {
		client.metrics.errorCount++
		return
	}
	client.metrics.successCount++
}

func (client *KIClient) MetricsSnapshot() HTTPMetricsSnapshot {
	client.metricsMu.Lock()
	metrics := client.metrics
	client.metricsMu.Unlock()

	snapshot := HTTPMetricsSnapshot{
		CallCount:     metrics.callCount,
		SuccessCount:  metrics.successCount,
		ErrorCount:    metrics.errorCount,
		TotalDuration: metrics.totalDuration,
		StartedAt:     metrics.startedAt,
		LastCallAt:    metrics.lastCallAt,
	}

	if snapshot.CallCount > 0 {
		snapshot.AverageTime = time.Duration(int64(snapshot.TotalDuration) / int64(snapshot.CallCount))
	}
	if !snapshot.StartedAt.IsZero() && !snapshot.LastCallAt.IsZero() && snapshot.LastCallAt.After(snapshot.StartedAt) {
		snapshot.Elapsed = snapshot.LastCallAt.Sub(snapshot.StartedAt)
	}
	if snapshot.Elapsed > 0 {
		snapshot.RPM = float64(snapshot.CallCount) / snapshot.Elapsed.Minutes()
	}

	return snapshot
}

func (client *KIClient) ResetMetrics() {
	client.metricsMu.Lock()
	defer client.metricsMu.Unlock()
	client.metrics = httpCallMetrics{}
}
