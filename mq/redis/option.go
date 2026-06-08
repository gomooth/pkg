package redis

import (
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/redis/go-redis/v9"
)

// ==================== Consumer 选项 ====================

// ConsumerOption 消费者配置选项
type ConsumerOption func(*consumerConfig)

// consumerConfig 消费者引擎配置（未导出）
type consumerConfig struct {
	logger       *slog.Logger
	redisOptions *redis.Options

	// 重试配置
	maxRetry       int
	backoff        retry.BackoffStrategy
	handlerTimeout time.Duration
	retryMode      types.RetryMode

	// 队列配置
	emptyQueueSleep time.Duration
	queuePrefix     string

	// 失败处理
	failedHandler types.FailedHandlerFunc

	// Panic 处理
	panicHandler func(any)

	// 预注册消费者
	consumers []ConsumerRegistration
}

// WithMaxRetry 设置最大重试次数（默认 3，0=不重试）
func WithMaxRetry(n int) ConsumerOption {
	return func(c *consumerConfig) {
		c.maxRetry = n
	}
}

// WithBackoff 设置退避策略（默认 ExponentialDelay{Base:1s, Max:5min}）
func WithBackoff(b retry.BackoffStrategy) ConsumerOption {
	return func(c *consumerConfig) {
		c.backoff = b
	}
}

// WithHandlerTimeout 设置单次 handler 调用的超时时间（默认 0，不限）
func WithHandlerTimeout(d time.Duration) ConsumerOption {
	return func(c *consumerConfig) {
		c.handlerTimeout = d
	}
}

// WithRetryMode 设置重试模式（默认 RetryModeSync）
func WithRetryMode(mode types.RetryMode) ConsumerOption {
	return func(c *consumerConfig) {
		c.retryMode = mode
	}
}

// WithPanicHandler 设置 panic 恢复后的回调函数
func WithPanicHandler(fn func(any)) ConsumerOption {
	return func(c *consumerConfig) {
		c.panicHandler = fn
	}
}

// WithEmptyQueueSleep 设置队列为空时的休眠时间（默认 1s）
func WithEmptyQueueSleep(d time.Duration) ConsumerOption {
	return func(c *consumerConfig) {
		if d > 0 {
			c.emptyQueueSleep = d
		}
	}
}

// WithFailedHandler 设置重试耗尽后的失败处理回调
func WithFailedHandler(fn types.FailedHandlerFunc) ConsumerOption {
	return func(c *consumerConfig) {
		c.failedHandler = fn
	}
}

// WithConsumers 批量预注册消费者
func WithConsumers(regs ...ConsumerRegistration) ConsumerOption {
	return func(c *consumerConfig) {
		c.consumers = append(c.consumers, regs...)
	}
}

// WithConsumer 预注册单个消费者
func WithConsumer(queue string, handler types.IHandler) ConsumerOption {
	return func(c *consumerConfig) {
		c.consumers = append(c.consumers, ConsumerRegistration{
			Queue:   queue,
			Handler: handler,
		})
	}
}

// WithConsumerLogger 设置消费者日志器
func WithConsumerLogger(l *slog.Logger) ConsumerOption {
	return func(c *consumerConfig) {
		c.logger = l
	}
}

// WithConsumerRedisConfig 设置自定义 Redis 连接配置
func WithConsumerRedisConfig(opt *redis.Options) ConsumerOption {
	return func(c *consumerConfig) {
		c.redisOptions = opt
	}
}

// WithQueuePrefix 设置队列名前缀（默认 "queue:"）
func WithQueuePrefix(prefix string) ConsumerOption {
	return func(c *consumerConfig) {
		c.queuePrefix = prefix
	}
}

// ==================== Producer 选项 ====================

// ProducerOption 生产者配置选项
type ProducerOption func(*producerConfig)

// producerConfig 生产者引擎配置（未导出）
type producerConfig struct {
	logger       *slog.Logger
	redisOptions *redis.Options
	queuePrefix  string
}

// WithProducerLogger 设置生产者日志器
func WithProducerLogger(l *slog.Logger) ProducerOption {
	return func(c *producerConfig) {
		c.logger = l
	}
}

// WithProducerRedisConfig 设置自定义 Redis 连接配置
func WithProducerRedisConfig(opt *redis.Options) ProducerOption {
	return func(c *producerConfig) {
		c.redisOptions = opt
	}
}

// WithProducerQueuePrefix 设置生产者队列名前缀（默认 "queue:"）
func WithProducerQueuePrefix(prefix string) ProducerOption {
	return func(c *producerConfig) {
		c.queuePrefix = prefix
	}
}
