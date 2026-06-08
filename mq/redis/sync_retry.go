package redis

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	mqretry "github.com/gomooth/pkg/mq/internal/retry"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
)

// retryStrategy 重试策略接口（未导出），与 consume.RetryStrategy 兼容
type retryStrategy interface {
	OnMessage(ctx context.Context, queue string, data []byte) error
}

// syncRetryStrategy 同步阻塞重试策略，内部委托给 mqretry.SyncStrategy
type syncRetryStrategy struct {
	inner   *mqretry.SyncStrategy
	handler types.IHandler
}

func newSyncRetryStrategy(
	handler types.IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	_ logutil.Logger,
	m *metrics.ConsumerMetrics,
) *syncRetryStrategy {
	backoffFn := mqretry.BackoffDelayFunc(func(attempt uint) time.Duration {
		return backoff.Delay(attempt)
	})

	return &syncRetryStrategy{
		handler: handler,
		inner: mqretry.NewSyncStrategy(mqretry.SyncConfig{
			MaxRetry: maxRetry,
			Backoff:  backoffFn,
			Metrics:  m,
		}),
	}
}

func (s *syncRetryStrategy) SetFailedHandler(fn types.FailedHandlerFunc) {
	s.inner.SetFailedHandler(fn)
}

func (s *syncRetryStrategy) SetDeadLetterHandler(h types.DeadLetterHandler) {
	s.inner.SetDeadLetterHandler(h)
}

func (s *syncRetryStrategy) SetTimeout(d time.Duration) {
	s.inner.SetTimeout(d)
}

func (s *syncRetryStrategy) OnMessage(ctx context.Context, queue string, data []byte) error {
	msg := types.NewRedisMessage(queue, data)
	err := s.inner.OnMessage(ctx, msg, s.handler.Handle)
	// 兼容旧行为：上下文取消时返回错误，耗尽时返回 nil
	if err != nil && ctx.Err() != nil {
		return err
	}
	return nil
}
