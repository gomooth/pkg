package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
)

// syncRetryStrategy 同步阻塞重试，从现有 ConsumeClaim 提取的逻辑
type syncRetryStrategy struct {
	consumerGroup   string
	maxRetry        uint
	backoff         retry.BackoffStrategy
	handler         func(ctx context.Context, topic string, msg []byte) error
	conf            *groupHandlerConf
	logger          Logger
	metrics         MetricsCallbacks
	maxTotalTimeout time.Duration
}

func newSyncRetryStrategy(cg string, conf *groupHandlerConf, backoff retry.BackoffStrategy, maxTotalTimeout time.Duration) *syncRetryStrategy {
	return &syncRetryStrategy{
		consumerGroup:   cg,
		maxRetry:        conf.MaxRetry,
		backoff:         backoff,
		handler:         conf.Handler,
		conf:            conf,
		maxTotalTimeout: maxTotalTimeout,
	}
}

func (s *syncRetryStrategy) OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	s.logger.Debug(fmt.Sprintf(
		"message claimed: cg=%q, topic=%q, time=%v, partition=%d, offset=%d",
		s.consumerGroup, msg.Topic, msg.Timestamp, msg.Partition, msg.Offset,
	))

	var lastErr error
	start := time.Now()
	for attempt := uint(0); attempt <= s.maxRetry; attempt++ {
		// 总超时检查
		if s.maxTotalTimeout > 0 && time.Since(start) > s.maxTotalTimeout {
			s.logger.Warn("sync retry total timeout exceeded, stopping retries",
				"topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset,
				"attempt", attempt, "maxTotalTimeout", s.maxTotalTimeout,
				"elapsed", time.Since(start))
			break
		}

		if err := s.handler(ctx, msg.Topic, msg.Value); err != nil {
			lastErr = err
			if attempt < s.maxRetry {
				// 记录重试指标
				if s.metrics.OnRetry != nil {
					s.metrics.OnRetry(ctx, msg.Topic)
				}
				delay := s.backoff.Delay(attempt)
				select {
				case <-session.Context().Done():
					return
				case <-time.After(delay):
				}
				continue
			}
		} else {
			lastErr = nil
			// 记录消费成功指标
			if s.metrics.OnConsume != nil {
				s.metrics.OnConsume(ctx, msg.Topic)
			}
			break
		}
	}

	if lastErr != nil {
		if s.conf.handleExhausted(ctx, s.consumerGroup, msg.Topic, msg.Value, lastErr, s.logger, s.metrics) == exhaustedHandled {
			session.MarkMessage(msg, "")
		}
		return
	}

	session.MarkMessage(msg, "")
}

func (s *syncRetryStrategy) SetSession(sarama.ConsumerGroupSession) {}

func (s *syncRetryStrategy) ClearSession() {}

func (s *syncRetryStrategy) OnShutdown(_ context.Context) {}

func (s *syncRetryStrategy) SetLogger(l Logger) {
	s.logger = l
}

func (s *syncRetryStrategy) SetMetrics(m MetricsCallbacks) {
	s.metrics = m
}
