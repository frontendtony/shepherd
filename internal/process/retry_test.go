package process

import (
	"testing"
	"time"

	"github.com/frontendtony/shepherd/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNextBackoff_ExponentialGrowth(t *testing.T) {
	cfg := config.RetryConfig{
		Enabled:           true,
		MaxAttempts:       5,
		InitialBackoff:    config.Duration(2 * time.Second),
		MaxBackoff:        config.Duration(60 * time.Second),
		BackoffMultiplier: 2,
	}

	// Attempt 0: ~2s, Attempt 1: ~4s, Attempt 2: ~8s
	b0 := nextBackoff(0, cfg)
	b1 := nextBackoff(1, cfg)
	b2 := nextBackoff(2, cfg)

	// Allow for jitter (+/- 10%).
	assert.InDelta(t, 2*time.Second, b0, float64(200*time.Millisecond))
	assert.InDelta(t, 4*time.Second, b1, float64(400*time.Millisecond))
	assert.InDelta(t, 8*time.Second, b2, float64(800*time.Millisecond))

	// Each should be roughly double the previous.
	assert.Greater(t, float64(b1), float64(b0))
	assert.Greater(t, float64(b2), float64(b1))
}

func TestNextBackoff_CappedAtMax(t *testing.T) {
	cfg := config.RetryConfig{
		Enabled:           true,
		MaxAttempts:       10,
		InitialBackoff:    config.Duration(2 * time.Second),
		MaxBackoff:        config.Duration(10 * time.Second),
		BackoffMultiplier: 2,
	}

	// Attempt 10 would be 2 * 2^10 = 2048s without cap.
	b := nextBackoff(10, cfg)

	// Should be capped at max + jitter.
	assert.LessOrEqual(t, float64(b), float64(11*time.Second)) // max + 10% jitter
}

func TestNextBackoff_Jitter(t *testing.T) {
	cfg := config.RetryConfig{
		Enabled:           true,
		InitialBackoff:    config.Duration(10 * time.Second),
		MaxBackoff:        config.Duration(60 * time.Second),
		BackoffMultiplier: 2,
	}

	// Run multiple times and check for variation.
	values := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		b := nextBackoff(0, cfg)
		values[b] = true
	}

	// With jitter, we should get multiple distinct values.
	assert.Greater(t, len(values), 1, "expected jitter to produce varying values")
}

func TestShouldRetry_Disabled(t *testing.T) {
	cfg := config.RetryConfig{
		Enabled:    false,
		MaxAttempts: 5,
	}

	assert.False(t, shouldRetry(0, cfg))
	assert.False(t, shouldRetry(1, cfg))
}

func TestShouldRetry_WithinLimit(t *testing.T) {
	cfg := config.RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
	}

	assert.True(t, shouldRetry(0, cfg))
	assert.True(t, shouldRetry(1, cfg))
	assert.True(t, shouldRetry(2, cfg))
	assert.False(t, shouldRetry(3, cfg))
	assert.False(t, shouldRetry(4, cfg))
}

func TestShouldRetry_InfiniteRetries(t *testing.T) {
	cfg := config.RetryConfig{
		Enabled:     true,
		MaxAttempts: 0,
	}

	assert.True(t, shouldRetry(0, cfg))
	assert.True(t, shouldRetry(100, cfg))
	assert.True(t, shouldRetry(999999, cfg))
}
