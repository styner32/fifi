package auth

import (
	"math/rand"
	"os"
	"strconv"
	"time"
)

// RetryPolicy configures the retry and backoff behavior for KIClient.
type RetryPolicy struct {
	MaxAttempts       int
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	JitterFraction    float64
	RetryableMsgCodes []string
}

// DefaultRetryPolicy returns the default retry policy, reading overrides from environment variables if present.
func DefaultRetryPolicy() RetryPolicy {
	maxAttempts := 5
	if v := os.Getenv("KIS_RETRY_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxAttempts = n
		}
	}

	baseDelay := 500 * time.Millisecond
	if v := os.Getenv("KIS_RETRY_BASE_DELAY_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			baseDelay = time.Duration(n) * time.Millisecond
		}
	}

	return RetryPolicy{
		MaxAttempts:       maxAttempts,
		BaseDelay:         baseDelay,
		MaxDelay:          8 * time.Second,
		JitterFraction:    0.3,
		RetryableMsgCodes: []string{rateLimitMessageCode},
	}
}

// CalculateBackoff computes the delay duration with exponential backoff and jitter for the given retry attempt.
func (p RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	// exponential: BaseDelay * 2^attempt
	multiplier := 1 << attempt
	if multiplier <= 0 || attempt > 30 {
		multiplier = 1 << 30
	}
	delay := time.Duration(float64(p.BaseDelay) * float64(multiplier))
	if delay > p.MaxDelay || delay < 0 {
		delay = p.MaxDelay
	}

	if p.JitterFraction > 0 {
		jitterRange := 2 * p.JitterFraction
		factor := (1.0 - p.JitterFraction) + (rand.Float64() * jitterRange)
		delay = time.Duration(float64(delay) * factor)
	}

	return delay
}

// IsRetryableMsgCode checks if a message code should trigger a retry.
func (p RetryPolicy) IsRetryableMsgCode(msgCode string) bool {
	for _, code := range p.RetryableMsgCodes {
		if code == msgCode {
			return true
		}
	}
	return false
}
