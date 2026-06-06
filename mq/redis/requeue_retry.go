package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/attempt_tracker"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// requeueRetryStrategy 再入队重试策略：Handle 失败后将消息重新 Push 回队列尾部
type requeueRetryStrategy struct {
	handler        IHandler
	maxRetry       int
	backoff        retry.BackoffStrategy
	client         *redis.Client
	queuePrefix    string
	logger         logutil.Logger
	metrics        *metrics.ConsumerMetrics
	failedHandler  FailedHandlerFunc
	deadLetter     DeadLetterHandler
	handlerTimeout time.Duration
	tracker        *attempt_tracker.AttemptTracker
}

func newRequeueRetryStrategy(
	handler IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	client *redis.Client,
	queuePrefix string,
	logger logutil.Logger,
	metrics *metrics.ConsumerMetrics,
) *requeueRetryStrategy {
	return &requeueRetryStrategy{
		handler:     handler,
		maxRetry:    maxRetry,
		backoff:     backoff,
		client:      client,
		queuePrefix: queuePrefix,
		logger:      logger,
		metrics:     metrics,
		tracker:     attempt_tracker.NewAttemptTracker(),
	}
}

func (s *requeueRetryStrategy) SetFailedHandler(fn FailedHandlerFunc) {
	s.failedHandler = fn
}

func (s *requeueRetryStrategy) SetDeadLetterHandler(h DeadLetterHandler) {
	s.deadLetter = h
}

func (s *requeueRetryStrategy) OnMessage(ctx context.Context, queue string, data []byte) error {
	// 应用 handler 超时
	hCtx, cancel := applyHandlerTimeout(ctx, s.handlerTimeout)
	err := s.handler.Handle(hCtx, queue, data)
	cancel()

	if err == nil {
		key := attempt_tracker.MessageKey(string(data))
		s.tracker.Remove(key)
		if s.metrics != nil {
			s.metrics.OnConsume()
		}
		return nil
	}

	// 处理失败，检查重试次数
	key := attempt_tracker.MessageKey(string(data))
	attempt := s.tracker.Increment(key)

	if attempt < s.maxRetry {
		// 退避等待后再入队
		delay := s.backoff.Delay(uint(attempt - 1))
		if s.logger != nil {
			s.logger.Warn("message handle failed, requeuing after backoff",
				"queue", queue,
				"attempt", attempt,
				"maxRetry", s.maxRetry,
				"delay", delay,
				"error", err,
			)
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		// RPUSH 回队列尾部
		queueKey := fmt.Sprintf("%s%s", s.queuePrefix, queue)
		if pushErr := s.client.RPush(ctx, queueKey, data).Err(); pushErr != nil {
			if s.logger != nil {
				s.logger.Error("failed to requeue message",
					"queue", queue, "error", pushErr)
			}
			// 再入队失败，走 exhausted 逻辑
			s.tracker.Remove(key)
			handleExhausted(ctx, queue, data, err,
				s.deadLetter, s.failedHandler, s.logger, s.metrics)
			return nil
		}

		if s.metrics != nil {
			s.metrics.OnRetry()
		}
		return nil
	}

	// 重试耗尽
	s.tracker.Remove(key)
	handleExhausted(ctx, queue, data, err,
		s.deadLetter, s.failedHandler, s.logger, s.metrics)
	return nil
}

// Close 停止 AttemptTracker 的后台清理 goroutine
func (s *requeueRetryStrategy) Close() {
	if s.tracker != nil {
		s.tracker.Close()
	}
}
