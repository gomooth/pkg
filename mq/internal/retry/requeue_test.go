package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/mq/internal/attempt_tracker"
	"github.com/gomooth/pkg/mq/internal/types"
)

func TestRequeueStrategy_Success(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 3,
		Backoff:  testBackoff,
		Tracker:  tracker,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestRequeueStrategy_RequeueOnFailure(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	var requeueCalls int
	requeueFn := func(_ context.Context, _ types.Message) error {
		requeueCalls++
		return nil
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 3,
		Backoff:  testBackoff,
		Tracker:  tracker,
		Requeue:  requeueFn,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("fail")
	})
	assert.NoError(t, err, "requeue strategy returns nil after requeue")
	assert.Equal(t, 1, requeueCalls, "requeue should be called once")
}

func TestRequeueStrategy_ExhaustedAfterMaxRetries(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	var requeueCalls int
	requeueFn := func(_ context.Context, _ types.Message) error {
		requeueCalls++
		return nil
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry:      3,
		Backoff:       testBackoff,
		Tracker:       tracker,
		Requeue:       requeueFn,
		FailedHandler: failedFn,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	handleErr := errors.New("fail")

	// First attempt: increment tracker to 1, 1 < 3 → requeue
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return handleErr
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, requeueCalls)
	assert.False(t, failedCalled, "should not be exhausted yet")

	// Second attempt: increment tracker to 2, 2 < 3 → requeue
	err = s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return handleErr
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, requeueCalls)
	assert.False(t, failedCalled, "should not be exhausted yet")

	// Third attempt: increment tracker to 3, 3 < 3 → false → exhausted
	err = s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return handleErr
	})
	assert.NoError(t, err)
	assert.True(t, failedCalled, "should be exhausted now")
}

func TestRequeueStrategy_NoTracker(t *testing.T) {
	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry:      3,
		FailedHandler: failedFn,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("fail")
	})
	assert.NoError(t, err)
	assert.True(t, failedCalled, "without tracker, should call FailedHandler immediately")
}

func TestRequeueStrategy_RequeueFail(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	requeueFn := func(_ context.Context, _ types.Message) error {
		return errors.New("requeue failed")
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry:      3,
		Tracker:       tracker,
		Requeue:       requeueFn,
		FailedHandler: failedFn,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("handle fail")
	})
	assert.NoError(t, err)
	assert.True(t, failedCalled, "should call FailedHandler when requeue fails")
}

func TestRequeueStrategy_Close(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 3,
		Tracker:  tracker,
	})

	// Close should not panic
	s.Close()

	// Double close should not panic
	s.Close()
}

func TestRequeueStrategy_DeadLetterOnExhausted(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	dl := &mockDeadLetterHandler{}

	requeueFn := func(_ context.Context, _ types.Message) error {
		return nil
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry:   2,
		Tracker:    tracker,
		Requeue:    requeueFn,
		DeadLetter: dl,
	})

	msg := types.NewRedisMessage("q", []byte("data"))
	handleErr := errors.New("fail")

	// Exhaust through max retries
	for i := 0; i < 3; i++ {
		_ = s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
			return handleErr
		})
	}
	assert.True(t, dl.called, "DeadLetterHandler should be called on exhaustion")
}
