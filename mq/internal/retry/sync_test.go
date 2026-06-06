package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/mq/internal/types"
)

func testBackoff(_ uint) time.Duration { return time.Millisecond }

func TestSyncStrategy_Success(t *testing.T) {
	s := NewSyncStrategy(SyncConfig{
		MaxRetry: 3,
		Backoff:  testBackoff,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestSyncStrategy_RetryThenSuccess(t *testing.T) {
	var attempts int
	s := NewSyncStrategy(SyncConfig{
		MaxRetry: 3,
		Backoff:  testBackoff,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestSyncStrategy_Exhausted(t *testing.T) {
	var failedCalled bool
	var failedErr error
	failedFn := func(_ context.Context, _ types.Message, err error) {
		failedCalled = true
		failedErr = err
	}

	s := NewSyncStrategy(SyncConfig{
		MaxRetry:      2,
		Backoff:       testBackoff,
		FailedHandler: failedFn,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("always fail")
	})
	assert.Error(t, err)
	assert.True(t, failedCalled, "FailedHandlerFunc should be called on exhaustion")
	assert.Equal(t, "always fail", failedErr.Error())
}

func TestSyncStrategy_DeadLetterOnExhausted(t *testing.T) {
	dl := &mockDeadLetterHandler{}

	s := NewSyncStrategy(SyncConfig{
		MaxRetry:   1,
		Backoff:    testBackoff,
		DeadLetter: dl,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("fail")
	})
	assert.Error(t, err)
	assert.True(t, dl.called, "DeadLetterHandler should be called on exhaustion")
}

func TestSyncStrategy_ContextCancelled(t *testing.T) {
	s := NewSyncStrategy(SyncConfig{
		MaxRetry: 10,
		Backoff:  testBackoff,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(ctx, msg, func(_ context.Context, _ types.Message) error {
		return errors.New("fail")
	})
	assert.True(t, errors.Is(err, context.Canceled))
}
