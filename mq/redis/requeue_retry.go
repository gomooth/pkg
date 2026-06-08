package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/attempt_tracker"
	mqretry "github.com/gomooth/pkg/mq/internal/retry"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/redis/go-redis/v9"
)

// requeueRetryStrategy 再入队重试策略，内部委托给 mqretry.RequeueStrategy
type requeueRetryStrategy struct {
	inner   *mqretry.RequeueStrategy
	handler types.IHandler
}

func newRequeueRetryStrategy(
	handler types.IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	client *redis.Client,
	queuePrefix string,
	_ logutil.Logger,
	m *metrics.ConsumerMetrics,
) *requeueRetryStrategy {
	backoffFn := mqretry.BackoffDelayFunc(func(attempt uint) time.Duration {
		return backoff.Delay(attempt)
	})

	tracker := attempt_tracker.NewAttemptTracker()

	requeueFn := func(ctx context.Context, msg types.Message) error {
		queueKey := fmt.Sprintf("%s%s", queuePrefix, msg.Queue)
		return client.RPush(ctx, queueKey, msg.Data).Err()
	}

	return &requeueRetryStrategy{
		handler: handler,
		inner: mqretry.NewRequeueStrategy(mqretry.RequeueConfig{
			MaxRetry: maxRetry,
			Backoff:  backoffFn,
			Tracker:  tracker,
			Metrics:  m,
			Requeue:  requeueFn,
		}),
	}
}

func (s *requeueRetryStrategy) SetFailedHandler(fn types.FailedHandlerFunc) {
	s.inner.SetFailedHandler(fn)
}

func (s *requeueRetryStrategy) SetDeadLetterHandler(h types.DeadLetterHandler) {
	s.inner.SetDeadLetterHandler(h)
}

func (s *requeueRetryStrategy) SetTimeout(d time.Duration) {
	s.inner.SetTimeout(d)
}

func (s *requeueRetryStrategy) OnMessage(ctx context.Context, queue string, data []byte) error {
	msg := types.NewRedisMessage(queue, data)
	err := s.inner.OnMessage(ctx, msg, s.handler.Handle)
	// 兼容旧行为：上下文取消时返回错误，其他情况返回 nil
	if err != nil && ctx.Err() != nil {
		return err
	}
	return nil
}

// Close 停止 AttemptTracker 的后台清理 goroutine
func (s *requeueRetryStrategy) Close() {
	s.inner.Close()
}
