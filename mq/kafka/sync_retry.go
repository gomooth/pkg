package kafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/kafka/internal"
)

// retryStrategy 重试策略接口（未导出）
type retryStrategy interface {
	OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage)
	SetSession(session sarama.ConsumerGroupSession)
	ClearSession()
	OnShutdown(ctx context.Context)
}

// exhaustedResult handleExhausted 的返回类型
type exhaustedResult int

const (
	exhaustedHandled exhaustedResult = iota // 已成功处理（可安全提交 offset）
	exhaustedFailed                         // 处理失败（不应提交 offset）
)

// syncRetryStrategy 同步阻塞重试策略
type syncRetryStrategy struct {
	consumerGroup   string
	handler         IHandler
	maxRetry        int
	backoff         retry.BackoffStrategy
	maxTotalTimeout time.Duration
	logger          internal.Logger
	metrics         *internal.ConsumerMetrics
	failedHandler   FailedHandlerFunc
	deadLetter      DeadLetterHandler
}

func newSyncRetryStrategy(
	cg string,
	handler IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	maxTotalTimeout time.Duration,
	logger internal.Logger,
	metrics *internal.ConsumerMetrics,
) *syncRetryStrategy {
	return &syncRetryStrategy{
		consumerGroup:   cg,
		handler:         handler,
		maxRetry:        maxRetry,
		backoff:         backoff,
		maxTotalTimeout: maxTotalTimeout,
		logger:          logger,
		metrics:         metrics,
	}
}

// SetFailedHandler 设置失败处理器
func (s *syncRetryStrategy) SetFailedHandler(fn FailedHandlerFunc) {
	s.failedHandler = fn
}

// SetDeadLetterHandler 设置死信处理器
func (s *syncRetryStrategy) SetDeadLetterHandler(h DeadLetterHandler) {
	s.deadLetter = h
}

func (s *syncRetryStrategy) OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	// P8 修复：移除 "message claimed" 重复日志

	var lastErr error
	var startTime time.Time
	if s.maxTotalTimeout > 0 {
		startTime = time.Now()
	}

	for attempt := 0; attempt <= s.maxRetry; attempt++ {
		// 总超时检查
		if s.maxTotalTimeout > 0 && attempt > 0 {
			if time.Since(startTime) >= s.maxTotalTimeout {
				if s.logger != nil {
					s.logger.Warn("sync retry total timeout exceeded",
						"topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset,
						"attempt", attempt, "maxTotalTimeout", s.maxTotalTimeout,
					)
				}
				break
			}
		}

		// context 取消检查
		if ctx.Err() != nil {
			return
		}

		err := s.handler.Handle(ctx, msg.Topic, msg.Value)
		if err == nil {
			session.MarkMessage(msg, "")
			if s.metrics != nil {
				s.metrics.OnConsume()
			}
			return
		}

		lastErr = err

		if attempt < s.maxRetry {
			if s.metrics != nil {
				s.metrics.OnRetry()
			}
			delay := s.backoff.Delay(uint(attempt))
			select {
			case <-time.After(delay):
			case <-session.Context().Done():
				return
			}
		}
	}

	// 重试耗尽
	result := handleExhausted(ctx, s.consumerGroup, msg.Topic, msg.Value, lastErr,
		s.deadLetter, s.failedHandler, s.logger, s.metrics)
	if result == exhaustedHandled {
		session.MarkMessage(msg, "")
	}
}

func (s *syncRetryStrategy) SetSession(sarama.ConsumerGroupSession) {}
func (s *syncRetryStrategy) ClearSession()                          {}
func (s *syncRetryStrategy) OnShutdown(_ context.Context)           {}

// handleExhausted 处理重试耗尽的消息（公共逻辑，各策略共享）
func handleExhausted(
	ctx context.Context,
	consumerGroup, topic string,
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
		if dlErr := deadLetter.OnDeadLetter(ctx, topic, message, lastErr); dlErr != nil {
			if logger != nil {
				logger.Error("dead letter handler failed, offset not committed",
					"topic", topic, "error", dlErr)
			}
			return exhaustedFailed
		}
		return exhaustedHandled
	}

	if failedHandler != nil {
		failedHandler(ctx, consumerGroup, topic, message, lastErr)
	} else if logger != nil {
		logger.Error("event handle failed after retries", "topic", topic, "error", lastErr)
	}
	return exhaustedHandled
}
