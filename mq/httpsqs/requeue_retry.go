package httpsqs

import (
	"context"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/httpsqs/internal"
	"github.com/gomooth/xerror"
)

// requeueRetryStrategy 再入队重试策略：Handle 失败后通过 HTTPSQS Put 将消息放回队列尾部
type requeueRetryStrategy struct {
	handler        IHandler
	maxRetry       int
	backoff        retry.BackoffStrategy
	client         httpsqs.IClient
	queueName      string
	logger         internal.Logger
	metrics        *internal.ConsumerMetrics
	failedHandler  FailedHandlerFunc
	deadLetter     DeadLetterHandler
	handlerTimeout time.Duration
	tracker        *internal.AttemptTracker
}

func newRequeueRetryStrategy(
	handler IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	client httpsqs.IClient,
	queueName string,
	logger internal.Logger,
	metrics *internal.ConsumerMetrics,
) *requeueRetryStrategy {
	return &requeueRetryStrategy{
		handler:   handler,
		maxRetry:  maxRetry,
		backoff:   backoff,
		client:    client,
		queueName: queueName,
		logger:    logger,
		metrics:   metrics,
		tracker:   internal.NewAttemptTracker(),
	}
}

func (s *requeueRetryStrategy) SetFailedHandler(fn FailedHandlerFunc) {
	s.failedHandler = fn
}

func (s *requeueRetryStrategy) SetDeadLetterHandler(h DeadLetterHandler) {
	s.deadLetter = h
}

func (s *requeueRetryStrategy) OnMessage(ctx context.Context, queue string, data string, pos int64) error {
	// 应用 handler 超时
	hCtx, cancel := applyHandlerTimeout(ctx, s.handlerTimeout)
	err := s.handler.Handle(hCtx, queue, data, pos)
	cancel()

	if err == nil {
		key := internal.MessageKey(data)
		s.tracker.Remove(key)
		if s.metrics != nil {
			s.metrics.OnConsume()
		}
		return nil
	}

	// 处理失败，检查重试次数
	key := internal.MessageKey(data)
	attempt := s.tracker.Increment(key)

	if attempt < s.maxRetry {
		// 退避等待后再入队
		delay := s.backoff.Delay(uint(attempt - 1))
		if s.logger != nil {
			s.logger.Warn("message handle failed, requeuing after backoff",
				"queue", queue,
				"pos", pos,
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

		// 通过 HTTPSQS Put 将消息放回队列尾部
		if _, pushErr := s.client.Put(ctx, s.queueName, data); pushErr != nil {
			if s.logger != nil {
				s.logger.Error("failed to requeue message",
					"queue", queue, "pos", pos, "error", pushErr)
			}
			// 再入队失败，走 exhausted 逻辑
			s.tracker.Remove(key)
			handleExhausted(ctx, queue, data, pos,
				xerror.Wrap(pushErr, "requeue failed"),
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
	handleExhausted(ctx, queue, data, pos, err,
		s.deadLetter, s.failedHandler, s.logger, s.metrics)
	return nil
}
