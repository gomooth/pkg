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
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestSyncRetryStrategy_Success(t *testing.T) {
	strategy := newSyncRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		}),
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
}

func TestSyncRetryStrategy_RetryThenSuccess(t *testing.T) {
	var attempt atomic.Int32

	strategy := newSyncRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			n := attempt.Add(1)
			if n < 3 {
				return errors.New("fail")
			}
			return nil
		}),
		5,
		&retry.FixedDelay{Wait: time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, int32(3), attempt.Load())
}

func TestSyncRetryStrategy_Exhausted(t *testing.T) {
	var failedCalled atomic.Int32

	strategy := newSyncRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return errors.New("always fail")
		}),
		2,
		&retry.FixedDelay{Wait: time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.SetFailedHandler(func(ctx context.Context, msg types.Message, err error) {
		failedCalled.Add(1)
	})

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err) // OnMessage 不返回 exhausted 错误
	assert.Equal(t, int32(1), failedCalled.Load())
}

func TestSyncRetryStrategy_ContextCancel(t *testing.T) {
	strategy := newSyncRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return errors.New("fail")
		}),
		100,
		&retry.FixedDelay{Wait: 10 * time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := strategy.OnMessage(ctx, "test", []byte("hello"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestSyncRetryStrategy_HandlerTimeout(t *testing.T) {
	strategy := newSyncRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}),
		0,
		&retry.FixedDelay{Wait: time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.SetTimeout(10 * time.Millisecond)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	// handlerTimeout 超时导致 Handle 返回 error，maxRetry=0 直接走 exhausted
	assert.NoError(t, err) // OnMessage 不返回 exhausted 错误
}

// httpsqsDeadLetterFunc 死信适配器，适配为 types.DeadLetterHandler
type httpsqsDeadLetterFunc func(ctx context.Context, msg types.Message, lastErr error) error

func (f httpsqsDeadLetterFunc) OnDeadLetter(ctx context.Context, msg types.Message, lastErr error) error {
	return f(ctx, msg, lastErr)
}

// nilLogger 返回一个丢弃所有输出的 logger
func nilLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}