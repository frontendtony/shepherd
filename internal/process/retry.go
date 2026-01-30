package process

import (
	"math"
	"math/rand"
	"time"

	"github.com/frontendtony/shepherd/internal/config"
)

// nextBackoff calculates the backoff duration for a given retry attempt.
// Uses exponential backoff with +/- 10% jitter.
func nextBackoff(attempt int, cfg config.RetryConfig) time.Duration {
	base := float64(cfg.InitialBackoff.Duration()) * math.Pow(cfg.BackoffMultiplier, float64(attempt))

	maxBackoff := float64(cfg.MaxBackoff.Duration())
	if base > maxBackoff {
		base = maxBackoff
	}

	// Add jitter: +/- 10%.
	jitter := base * 0.1
	base = base - jitter + (rand.Float64() * 2 * jitter)

	return time.Duration(base)
}

// shouldRetry returns true if the process should be retried.
func shouldRetry(attempt int, cfg config.RetryConfig) bool {
	if !cfg.Enabled {
		return false
	}
	// MaxAttempts 0 means infinite retries.
	if cfg.MaxAttempts == 0 {
		return true
	}
	return attempt < cfg.MaxAttempts
}
