package httpsqs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/httpsqs/internal"
	"github.com/stretchr/testify/assert"
)

func TestSyncRetryStrategy_Success(t *testing.T) {
	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return nil
		}),
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
}

func TestSyncRetryStrategy_RetryThenSuccess(t *testing.T) {
	var attempt atomic.Int32

	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			n := attempt.Add(1)
			if n < 3 {
				return errors.New("fail")
			}
			return nil
		}),
		5,
		&retry.FixedDelay{Wait: time.Millisecond},
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), attempt.Load())
}

func TestSyncRetryStrategy_Exhausted(t *testing.T) {
	var failedCalled atomic.Int32

	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("always fail")
		}),
		2,
		&retry.FixedDelay{Wait: time.Millisecond},
		internal.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.SetFailedHandler(func(ctx context.Context, queue string, data string, pos int64, err error) {
		failedCalled.Add(1)
	})

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err) // OnMessage 不返回 exhausted 错误
	assert.Equal(t, int32(1), failedCalled.Load())
}

func TestSyncRetryStrategy_ContextCancel(t *testing.T) {
	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("fail")
		}),
		100,
		&retry.FixedDelay{Wait: 10 * time.Millisecond},
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := strategy.OnMessage(ctx, "test", "hello", 1)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestSyncRetryStrategy_HandlerTimeout(t *testing.T) {
	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}),
		0,
		&retry.FixedDelay{Wait: time.Millisecond},
		internal.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.handlerTimeout = 10 * time.Millisecond

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	// handlerTimeout 超时导致 Handle 返回 error，maxRetry=0 直接走 exhausted
	assert.NoError(t, err) // OnMessage 不返回 exhausted 错误
}

func TestHandleExhausted_WithDeadLetter(t *testing.T) {
	var dlCalled atomic.Int32

	result := handleExhausted(
		context.Background(),
		"test",
		"hello",
		1,
		errors.New("fail"),
		httpsqsDeadLetterFunc(func(ctx context.Context, queue string, data string, pos int64, lastErr error) error {
			dlCalled.Add(1)
			return nil
		}),
		nil,
		nil,
		nil,
	)

	assert.Equal(t, exhaustedContinue, result)
	assert.Equal(t, int32(1), dlCalled.Load())
}

func TestHandleExhausted_WithFailedHandler(t *testing.T) {
	var fhCalled atomic.Int32

	result := handleExhausted(
		context.Background(),
		"test",
		"hello",
		1,
		errors.New("fail"),
		nil,
		func(ctx context.Context, queue string, data string, pos int64, err error) {
			fhCalled.Add(1)
		},
		nil,
		nil,
	)

	assert.Equal(t, exhaustedContinue, result)
	assert.Equal(t, int32(1), fhCalled.Load())
}

// httpsqsDeadLetterFunc 死信适配器
type httpsqsDeadLetterFunc func(ctx context.Context, queue string, data string, pos int64, lastErr error) error

func (f httpsqsDeadLetterFunc) OnDeadLetter(ctx context.Context, queue string, data string, pos int64, lastErr error) error {
	return f(ctx, queue, data, pos, lastErr)
}

// nilLogger 返回一个丢弃所有输出的 logger
func nilLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
