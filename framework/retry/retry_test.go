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
