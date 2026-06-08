package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFixedDelay(t *testing.T) {
	s := &FixedDelay{Wait: 500 * time.Millisecond}
	assert.Equal(t, 500*time.Millisecond, s.Delay(0))
	assert.Equal(t, 500*time.Millisecond, s.Delay(5))
	assert.Equal(t, 500*time.Millisecond, s.Delay(100))
}

func TestLinearDelay(t *testing.T) {
	s := &LinearDelay{Base: time.Second}
	assert.Equal(t, 1*time.Second, s.Delay(0))
	assert.Equal(t, 2*time.Second, s.Delay(1))
	assert.Equal(t, 3*time.Second, s.Delay(2))
	assert.Equal(t, 10*time.Second, s.Delay(9))
}

func TestExponentialDelay(t *testing.T) {
	s := &ExponentialDelay{Base: time.Second, Max: 5 * time.Minute}
	assert.Equal(t, 1*time.Second, s.Delay(0))
	assert.Equal(t, 2*time.Second, s.Delay(1))
	assert.Equal(t, 4*time.Second, s.Delay(2))
	assert.Equal(t, 8*time.Second, s.Delay(3))
	assert.Equal(t, 16*time.Second, s.Delay(4))
}

func TestExponentialDelay_MaxCap(t *testing.T) {
	s := &ExponentialDelay{Base: time.Second, Max: 10 * time.Second}
	// 2^10 = 1024 seconds > 10s cap
	assert.Equal(t, 10*time.Second, s.Delay(10))
	assert.Equal(t, 10*time.Second, s.Delay(30))
}

func TestExponentialDelay_OverflowProtection(t *testing.T) {
	t.Run("Max=0 Base=1s high attempt does not return negative", func(t *testing.T) {
		d := &ExponentialDelay{Base: time.Second, Max: 0}
		for attempt := uint(0); attempt <= 63; attempt++ {
			result := d.Delay(attempt)
			assert.GreaterOrEqual(t, result, time.Duration(0), "attempt=%d should not return negative duration", attempt)
		}
	})

	t.Run("Max=5s limits to Max", func(t *testing.T) {
		d := &ExponentialDelay{Base: time.Second, Max: 5 * time.Second}
		result := d.Delay(10)
		assert.LessOrEqual(t, result, 5*time.Second)
	})

	t.Run("Max=0 Base=0 returns 0", func(t *testing.T) {
		d := &ExponentialDelay{Base: 0, Max: 0}
		result := d.Delay(5)
		assert.Equal(t, time.Duration(0), result)
	})

	t.Run("overflow falls back to Max when Max>0", func(t *testing.T) {
		d := &ExponentialDelay{Base: time.Hour, Max: time.Minute}
		result := d.Delay(63)
		assert.Equal(t, time.Minute, result, "overflow should fall back to Max")
	})
}

func TestDo_SuccessFirstAttempt(t *testing.T) {
	ctx := context.Background()
	err := Do(ctx, Config{MaxAttempts: 3, Strategy: &FixedDelay{Wait: time.Millisecond}}, func(attempt uint) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	var attempts uint
	err := Do(ctx, Config{MaxAttempts: 5, Strategy: &FixedDelay{Wait: time.Millisecond}}, func(attempt uint) error {
		attempts++
		if attempt < 2 {
			return errors.New("temporary error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, uint(3), attempts)
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	var attempts uint
	err := Do(ctx, Config{MaxAttempts: 3, Strategy: &FixedDelay{Wait: time.Millisecond}}, func(attempt uint) error {
		attempts++
		return errors.New("persistent error")
	})
	assert.Error(t, err)
	assert.Equal(t, uint(3), attempts)
	assert.Equal(t, "persistent error", err.Error())
}

func TestDo_InfiniteRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var attempts uint
	err := Do(ctx, Config{MaxAttempts: InfiniteRetry, Strategy: &FixedDelay{Wait: time.Millisecond}}, func(attempt uint) error {
		attempts++
		return errors.New("keep retrying")
	})
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.True(t, attempts > 1)
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, Config{MaxAttempts: InfiniteRetry, Strategy: &FixedDelay{Wait: time.Millisecond}}, func(attempt uint) error {
		return errors.New("always fail")
	})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestDo_NilStrategyDefaultsToFixed(t *testing.T) {
	ctx := context.Background()
	err := Do(ctx, Config{MaxAttempts: 1, Strategy: nil}, func(attempt uint) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestDo_AttemptNumber(t *testing.T) {
	ctx := context.Background()
	var seenAttempts []uint
	_ = Do(ctx, Config{MaxAttempts: 3, Strategy: &FixedDelay{Wait: time.Millisecond}}, func(attempt uint) error {
		seenAttempts = append(seenAttempts, attempt)
		return errors.New("fail")
	})
	assert.Equal(t, []uint{0, 1, 2}, seenAttempts)
}

// --- ExponentialDelay boundary tests ---

func TestExponentialDelay_WithJitter(t *testing.T) {
	s := &ExponentialDelay{Base: time.Second, Max: 10 * time.Second, Jitter: true}
	// With Jitter and d > 0: result is in [d/2, d) due to Equal Jitter
	for attempt := uint(0); attempt < 5; attempt++ {
		result := s.Delay(attempt)
		assert.GreaterOrEqual(t, result, time.Duration(0), "attempt=%d jitter result should be >= 0", attempt)
	}
	// Specific check: attempt=0, base=1s, no Max cap, so d=1s before jitter
	// After jitter: d/2 + rand(0, d/2) => result in [500ms, 1s)
	result := s.Delay(0)
	assert.GreaterOrEqual(t, result, 500*time.Millisecond, "jitter result should be >= half of delay")
	assert.LessOrEqual(t, result, time.Second, "jitter result should be <= original delay")
}

func TestExponentialDelay_NoJitter(t *testing.T) {
	s := &ExponentialDelay{Base: 100 * time.Millisecond, Max: 0}
	// Without jitter: exact values
	assert.Equal(t, 100*time.Millisecond, s.Delay(0))  // 100ms * 2^0
	assert.Equal(t, 200*time.Millisecond, s.Delay(1))  // 100ms * 2^1
	assert.Equal(t, 400*time.Millisecond, s.Delay(2))  // 100ms * 2^2
	assert.Equal(t, 800*time.Millisecond, s.Delay(3))  // 100ms * 2^3
}

func TestExponentialDelay_HighAttempt_ShiftClamped(t *testing.T) {
	s := &ExponentialDelay{Base: time.Nanosecond, Max: 0}
	// shift is clamped to 30, so attempt=30 and attempt=100 should produce same result
	r30 := s.Delay(30)
	r100 := s.Delay(100)
	r31 := s.Delay(31)
	assert.Equal(t, r30, r100, "attempt > 30 should produce same result as attempt=30")
	assert.Equal(t, r30, r31, "attempt=31 should produce same result as attempt=30 (shift clamped)")
	assert.GreaterOrEqual(t, r30, time.Duration(0), "high attempt should not produce negative duration")
}

func TestExponentialDelay_ZeroValue(t *testing.T) {
	s := &ExponentialDelay{}
	// Zero value should not panic
	assert.NotPanics(t, func() {
		result := s.Delay(0)
		assert.Equal(t, time.Duration(0), result)
	})
	assert.NotPanics(t, func() {
		result := s.Delay(10)
		assert.Equal(t, time.Duration(0), result)
	})
}

func TestExponentialDelay_BaseGreaterThanMax(t *testing.T) {
	// When Base > Max, even attempt=0 should be clamped to Max
	s := &ExponentialDelay{Base: 10 * time.Second, Max: time.Second}
	assert.Equal(t, time.Second, s.Delay(0), "Base > Max should be clamped to Max at attempt=0")
	assert.Equal(t, time.Second, s.Delay(5), "Base > Max should be clamped to Max at higher attempts")
}

func TestExponentialDelay_JitterWithZeroDelay(t *testing.T) {
	// When d == 0 and Jitter is true, the `d > 0` check should skip jitter
	s := &ExponentialDelay{Base: 0, Max: 0, Jitter: true}
	assert.Equal(t, time.Duration(0), s.Delay(0), "jitter with zero delay should return 0")
}

func TestExponentialDelay_OverflowFallback(t *testing.T) {
	// Max=0 with overflow: should fall back to 1<<30 * Base
	s := &ExponentialDelay{Base: time.Second, Max: 0}
	// attempt=63 will cause overflow; without Max, falls back to 1<<30 * Base
	result := s.Delay(63)
	expected := time.Duration(1<<30) * time.Second
	assert.Equal(t, expected, result, "overflow with Max=0 should fall back to 1<<30 * Base")
}

// --- Do boundary tests ---

func TestDo_RetryIfReturnsFalse(t *testing.T) {
	ctx := context.Background()
	var attempts uint
	err := Do(ctx, Config{
		MaxAttempts: 5,
		Strategy:    &FixedDelay{Wait: time.Millisecond},
		RetryIf: func(err error) bool {
			return false // never retry
		},
	}, func(attempt uint) error {
		attempts++
		return errors.New("some error")
	})
	assert.Error(t, err)
	assert.Equal(t, uint(1), attempts, "should not retry when RetryIf returns false")
	assert.Equal(t, "some error", err.Error())
}

func TestDo_RetryIfReturnsTrue(t *testing.T) {
	ctx := context.Background()
	var attempts uint
	err := Do(ctx, Config{
		MaxAttempts: 3,
		Strategy:    &FixedDelay{Wait: time.Millisecond},
		RetryIf: func(err error) bool {
			return true // always retry
		},
	}, func(attempt uint) error {
		attempts++
		return errors.New("retryable error")
	})
	assert.Error(t, err)
	assert.Equal(t, uint(3), attempts, "should retry when RetryIf returns true")
}

func TestDo_MaxAttemptsZero_UsesDefault(t *testing.T) {
	ctx := context.Background()
	var attempts uint
	err := Do(ctx, Config{
		MaxAttempts: 0, // should use default (10)
		Strategy:    &FixedDelay{Wait: time.Millisecond},
	}, func(attempt uint) error {
		attempts++
		return errors.New("fail")
	})
	assert.Error(t, err)
	assert.Equal(t, uint(10), attempts, "MaxAttempts=0 should use default value of 10")
}

func TestDo_ContextCancelledBeforeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := Do(ctx, Config{
		MaxAttempts: 5,
		Strategy:    &FixedDelay{Wait: time.Millisecond},
	}, func(attempt uint) error {
		return errors.New("fail")
	})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err, "should return context error when cancelled before loop starts")
}

func TestDo_RetryIfSelective(t *testing.T) {
	ctx := context.Background()
	var attempts uint
	retryableErr := errors.New("retryable")
	nonRetryableErr := errors.New("fatal")

	err := Do(ctx, Config{
		MaxAttempts: 10,
		Strategy:    &FixedDelay{Wait: time.Millisecond},
		RetryIf: func(err error) bool {
			return err == retryableErr
		},
	}, func(attempt uint) error {
		attempts++
		if attempt < 2 {
			return retryableErr
		}
		return nonRetryableErr
	})
	assert.Equal(t, nonRetryableErr, err)
	assert.Equal(t, uint(3), attempts, "should retry on retryable error but stop on non-retryable")
}
