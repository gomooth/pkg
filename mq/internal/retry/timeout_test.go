package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplyTimeout_NoTimeout(t *testing.T) {
	var called bool
	err := ApplyTimeout(context.Background(), 0, func(_ context.Context) error {
		called = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestApplyTimeout_NegativeTimeout(t *testing.T) {
	var called bool
	err := ApplyTimeout(context.Background(), -1*time.Second, func(_ context.Context) error {
		called = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestApplyTimeout_WithinTimeout(t *testing.T) {
	var called bool
	err := ApplyTimeout(context.Background(), 5*time.Second, func(_ context.Context) error {
		called = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestApplyTimeout_ExceedsTimeout(t *testing.T) {
	err := ApplyTimeout(context.Background(), 50*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected DeadlineExceeded, got %v", err)
}

func TestApplyTimeout_PropagatesError(t *testing.T) {
	expectedErr := errors.New("custom error")
	err := ApplyTimeout(context.Background(), time.Second, func(_ context.Context) error {
		return expectedErr
	})
	assert.Equal(t, expectedErr, err)
}
