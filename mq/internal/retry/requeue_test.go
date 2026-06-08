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

// TestRequeueStrategy_SetFailedHandler 验证通过 SetFailedHandler 设置的回调在重试耗尽时被调用
func TestRequeueStrategy_SetFailedHandler(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 2,
		Tracker:  tracker,
		Requeue:  func(_ context.Context, _ types.Message) error { return nil },
	})
	s.SetFailedHandler(failedFn)

	msg := types.NewRedisMessage("q", []byte("data"))
	handleErr := errors.New("fail")

	// 耗尽重试次数
	for i := 0; i < 3; i++ {
		_ = s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
			return handleErr
		})
	}
	assert.True(t, failedCalled, "SetFailedHandler 设置的回调应被调用")
}

// TestRequeueStrategy_SetDeadLetterHandler 验证死信处理器优先于 FailedHandler
func TestRequeueStrategy_SetDeadLetterHandler(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	dl := &mockDeadLetterHandler{}
	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 1,
		Tracker:  tracker,
		Requeue:  func(_ context.Context, _ types.Message) error { return nil },
	})
	s.SetDeadLetterHandler(dl)
	s.SetFailedHandler(failedFn)

	msg := types.NewRedisMessage("q", []byte("data"))
	handleErr := errors.New("fail")

	// 耗尽重试次数
	for i := 0; i < 2; i++ {
		_ = s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
			return handleErr
		})
	}
	assert.True(t, dl.called, "SetDeadLetterHandler 设置的处理器应被调用")
	assert.False(t, failedCalled, "死信处理器应优先于 FailedHandler")
}

// TestRequeueStrategy_SetTimeout 验证超时生效，处理函数超过超时时间后返回 context.DeadlineExceeded
func TestRequeueStrategy_SetTimeout(t *testing.T) {
	tracker := attempt_tracker.NewAttemptTracker(
		attempt_tracker.WithMaxAge(time.Minute),
		attempt_tracker.WithCleanInterval(time.Hour),
	)
	defer tracker.Close()

	s := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 0,
		Tracker:  tracker,
	})
	s.SetTimeout(10 * time.Millisecond)

	msg := types.NewRedisMessage("q", []byte("data"))
	_ = s.OnMessage(context.Background(), msg, func(ctx context.Context, _ types.Message) error {
		// 模拟耗时处理，超过超时时间
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	// RequeueStrategy 返回 nil（始终不返回错误），但超时应使 handler 收到 context.DeadlineExceeded
	// 验证方式：无 tracker 且 MaxRetry=0 时直接走 HandleExhausted
	s2 := NewRequeueStrategy(RequeueConfig{
		MaxRetry: 0,
	})
	s2.SetTimeout(10 * time.Millisecond)

	var failedErr error
	s2.SetFailedHandler(func(_ context.Context, _ types.Message, err error) {
		failedErr = err
	})

	_ = s2.OnMessage(context.Background(), msg, func(ctx context.Context, _ types.Message) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	assert.True(t, errors.Is(failedErr, context.DeadlineExceeded), "SetTimeout 设置的超时应生效")
}
