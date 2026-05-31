package redis

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/redis/internal"
)

// retryStrategy 重试策略接口（未导出）
type retryStrategy interface {
	OnMessage(ctx context.Context, queue string, data []byte) error
}

// exhaustedResult handleExhausted 的返回类型
type exhaustedResult int

const (
	exhaustedContinue exhaustedResult = iota // 继续消费下一条
	exhaustedStop                            // 停止消费（严重错误）
)

// handleExhausted 处理重试耗尽的消息（公共逻辑，各策略共享）
func handleExhausted(
	ctx context.Context,
	queue string,
	message []byte,
	lastErr error,
	deadLetter DeadLetterHandler,
	failedHandler FailedHandlerFunc,
	logger internal.Logger,
	metrics *internal.ConsumerMetrics,
) exhaustedResult {
	if metrics != nil {
		metrics.OnDeadLetter()
	}

	if deadLetter != nil {
		if dlErr := deadLetter.OnDeadLetter(ctx, queue, message, lastErr); dlErr != nil {
			if logger != nil {
				logger.Error("dead letter handler failed",
					"queue", queue, "error", dlErr)
			}
			return exhaustedContinue
		}
		return exhaustedContinue
	}

	if failedHandler != nil {
		failedHandler(ctx, queue, message, lastErr)
	} else if logger != nil {
		logger.Error("event handle failed after retries", "queue", queue, "error", lastErr)
	}
	return exhaustedContinue
}

// applyHandlerTimeout 为 handler 调用添加超时控制
func applyHandlerTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

// syncRetryStrategy 同步阻塞重试策略
type syncRetryStrategy struct {
	handler        IHandler
	maxRetry       int
	backoff        retry.BackoffStrategy
	logger         internal.Logger
	metrics        *internal.ConsumerMetrics
	failedHandler  FailedHandlerFunc
	deadLetter     DeadLetterHandler
	handlerTimeout time.Duration
}

func newSyncRetryStrategy(
	handler IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	logger internal.Logger,
	metrics *internal.ConsumerMetrics,
) *syncRetryStrategy {
	return &syncRetryStrategy{
		handler:  handler,
		maxRetry: maxRetry,
		backoff:  backoff,
		logger:   logger,
		metrics:  metrics,
	}
}

func (s *syncRetryStrategy) SetFailedHandler(fn FailedHandlerFunc) {
	s.failedHandler = fn
}

func (s *syncRetryStrategy) SetDeadLetterHandler(h DeadLetterHandler) {
	s.deadLetter = h
}

func (s *syncRetryStrategy) OnMessage(ctx context.Context, queue string, data []byte) error {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetry; attempt++ {
		// context 取消检查
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 应用 handler 超时
		hCtx, cancel := applyHandlerTimeout(ctx, s.handlerTimeout)
		err := s.handler.Handle(hCtx, queue, data)
		cancel()

		if err == nil {
			if s.metrics != nil {
				s.metrics.OnConsume()
			}
			return nil
		}

		lastErr = err

		if attempt < s.maxRetry {
			if s.metrics != nil {
				s.metrics.OnRetry()
			}
			delay := s.backoff.Delay(uint(attempt))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// 重试耗尽
	handleExhausted(ctx, queue, data, lastErr,
		s.deadLetter, s.failedHandler, s.logger, s.metrics)
	return nil
}
