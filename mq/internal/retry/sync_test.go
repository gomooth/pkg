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

// TestSyncStrategy_SetFailedHandler 验证通过 SetFailedHandler 设置的回调在重试耗尽时被调用
func TestSyncStrategy_SetFailedHandler(t *testing.T) {
	var failedCalled bool
	var failedErr error
	failedFn := func(_ context.Context, _ types.Message, err error) {
		failedCalled = true
		failedErr = err
	}

	s := NewSyncStrategy(SyncConfig{
		MaxRetry: 1,
		Backoff:  testBackoff,
	})
	s.SetFailedHandler(failedFn)

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("always fail")
	})
	assert.Error(t, err)
	assert.True(t, failedCalled, "SetFailedHandler 设置的回调应被调用")
	assert.Equal(t, "always fail", failedErr.Error())
}

// TestSyncStrategy_SetDeadLetterHandler 验证死信处理器优先于 FailedHandler
func TestSyncStrategy_SetDeadLetterHandler(t *testing.T) {
	dl := &mockDeadLetterHandler{}
	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	s := NewSyncStrategy(SyncConfig{
		MaxRetry: 1,
		Backoff:  testBackoff,
	})
	s.SetDeadLetterHandler(dl)
	s.SetFailedHandler(failedFn)

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(_ context.Context, _ types.Message) error {
		return errors.New("fail")
	})
	assert.Error(t, err)
	assert.True(t, dl.called, "SetDeadLetterHandler 设置的处理器应被调用")
	assert.False(t, failedCalled, "死信处理器应优先于 FailedHandler")
}

// TestSyncStrategy_SetTimeout 验证超时生效，处理函数超过超时时间后返回 context.DeadlineExceeded
func TestSyncStrategy_SetTimeout(t *testing.T) {
	s := NewSyncStrategy(SyncConfig{
		MaxRetry: 0,
	})
	s.SetTimeout(10 * time.Millisecond)

	msg := types.NewRedisMessage("q", []byte("data"))
	err := s.OnMessage(context.Background(), msg, func(ctx context.Context, _ types.Message) error {
		// 模拟耗时处理，超过超时时间
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "SetTimeout 设置的超时应生效")
}
