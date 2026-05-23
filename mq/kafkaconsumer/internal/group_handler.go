package internal

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
)

type groupHandler struct {
	consumerGroup string

	maxRetry      uint
	backoff       retry.BackoffStrategy
	handler       func(ctx context.Context, topic string, msg []byte) error
	failedHandler func(ctx context.Context, consumerGroup, topic string, msg []byte, err error)

	deadLetterHandler func(ctx context.Context, topic string, msg []byte, err error) error

	logger *slog.Logger

	retryStrategy RetryStrategy
	retryMode     RetryMode

	syncRetryMaxTotalTimeout time.Duration
	handlerTimeout           time.Duration
}

type groupHandlerConf struct {
	Logger *slog.Logger

	Handler           func(ctx context.Context, topic string, msg []byte) error
	FailedHandler     func(ctx context.Context, consumerGroup, topic string, msg []byte, err error)
	DeadLetterHandler func(ctx context.Context, topic string, msg []byte, err error) error

	MaxRetry uint
	Backoff  retry.BackoffStrategy

	// 新增：重试模式配置
	RetryMode         RetryMode
	RetryWorkers      int
	RetryMaxQueueSize int
	RedisStore        RedisRetryStore

	// 新增：Metrics 回调
	Metrics MetricsCallbacks

	// 同步重试总超时上限，0 表示不限
	SyncRetryMaxTotalTimeout time.Duration

	// 单次 handler 调用超时，0 表示不限
	HandlerTimeout time.Duration
}

// MetricsCallbacks 指标回调函数，由外部包注入
type MetricsCallbacks struct {
	OnConsume    func(ctx context.Context, topic string)
	OnRetry      func(ctx context.Context, topic string)
	OnDeadLetter func(ctx context.Context, topic string)
}

// exhaustedResult handleExhausted 的返回类型
type exhaustedResult int

const (
	exhaustedHandled exhaustedResult = iota // 已成功处理（可安全提交 offset）
	exhaustedFailed                         // 死信处理失败（不应提交 offset）
)

// handleExhausted 处理重试耗尽的消息，返回是否已成功处理可安全提交 offset。
// 此方法由各重试策略共享调用，避免重复实现。
func (c *groupHandlerConf) handleExhausted(ctx context.Context, cg string, topic string, msg []byte, lastErr error, logger Logger, metrics MetricsCallbacks) exhaustedResult {
	// 记录死信指标
	if metrics.OnDeadLetter != nil {
		metrics.OnDeadLetter(ctx, topic)
	}

	if c.DeadLetterHandler != nil {
		if dlErr := c.DeadLetterHandler(ctx, topic, msg, lastErr); dlErr != nil {
			logger.Error("dead letter handler failed, offset not committed",
				"topic", topic, "error", dlErr)
			return exhaustedFailed
		}
		return exhaustedHandled
	} else if c.FailedHandler != nil {
		c.FailedHandler(ctx, cg, topic, msg, lastErr)
	} else {
		logger.Error("event handle failed after retries", "topic", topic, "error", lastErr)
	}
	return exhaustedHandled
}

func newConsumerGroupHandler(cg string, conf *groupHandlerConf) *groupHandler {
	backoff := conf.Backoff
	if backoff == nil {
		backoff = &retry.ExponentialDelay{Base: 10 * time.Second, Max: 5 * time.Minute}
	}

	numWorkers := conf.RetryWorkers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
		if numWorkers < 1 {
			numWorkers = 1
		}
	}

	var strategy RetryStrategy
	switch conf.RetryMode {
	case RetryModeAsyncWatermark:
		strategy = newAsyncWatermarkRetry(cg, conf, backoff, numWorkers)
	case RetryModeAsyncRedis:
		if conf.RedisStore == nil {
			// Redis 模式必须提供 RedisStore，降级为 Sync
			strategy = newSyncRetryStrategy(cg, conf, backoff, conf.SyncRetryMaxTotalTimeout)
		} else {
			strategy = newAsyncRedisRetry(cg, conf, backoff, numWorkers, conf.RedisStore)
		}
	default: // RetryModeSync
		if conf.MaxRetry > 1 && conf.Logger != nil {
			conf.Logger.Warn("sync retry mode may block partition for extended periods, "+
				"consider using RetryModeAsyncWatermark or RetryModeAsyncRedis for production",
				"maxRetry", conf.MaxRetry, "backoff", backoff,
				"syncRetryMaxTotalTimeout", conf.SyncRetryMaxTotalTimeout)
		}
		strategy = newSyncRetryStrategy(cg, conf, backoff, conf.SyncRetryMaxTotalTimeout)
	}

	// 注入 Metrics 回调
	strategy.SetMetrics(conf.Metrics)

	// 注入 Logger
	if conf.Logger != nil {
		strategy.SetLogger(newSlogLogger(conf.Logger))
	}

	return &groupHandler{
		consumerGroup:            cg,
		handler:                  conf.Handler,
		failedHandler:            conf.FailedHandler,
		deadLetterHandler:        conf.DeadLetterHandler,
		logger:                   conf.Logger,
		maxRetry:                 conf.MaxRetry,
		backoff:                  backoff,
		retryStrategy:            strategy,
		retryMode:                conf.RetryMode,
		syncRetryMaxTotalTimeout: conf.SyncRetryMaxTotalTimeout,
		handlerTimeout:           conf.HandlerTimeout,
	}
}

func (c *groupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.retryStrategy.SetSession(session)
	return nil
}

func (c *groupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	c.retryStrategy.ClearSession()
	return nil
}

// Shutdown 优雅关闭重试策略，排空重试队列
func (c *groupHandler) Shutdown(ctx context.Context) {
	c.retryStrategy.OnShutdown(ctx)
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				c.logger.Debug("message channel was closed")
				return nil
			}

			c.logger.Debug(fmt.Sprintf(
				"message claimed: cg=%q, topic=%q, time=%v, partition=%d, offset=%d",
				c.consumerGroup, msg.Topic, msg.Timestamp, msg.Partition, msg.Offset,
			))

			// 为每条消息的 handler 调用添加超时 context
			msgCtx := session.Context()
			if c.handlerTimeout > 0 {
				var cancel context.CancelFunc
				msgCtx, cancel = context.WithTimeout(msgCtx, c.handlerTimeout)
				c.retryStrategy.OnMessage(msgCtx, session, msg)
				cancel()
			} else {
				c.retryStrategy.OnMessage(msgCtx, session, msg)
			}

		case <-session.Context().Done():
			return nil
		}
	}
}
