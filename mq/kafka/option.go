package kafka

import (
	"log/slog"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
)

// ==================== Consumer 选项 ====================

// ConsumerOption 消费者配置选项
type ConsumerOption func(*consumerConfig)

// consumerConfig 消费者引擎配置（未导出）
type consumerConfig struct {
	// 基础配置
	logger       *slog.Logger
	timeout      time.Duration
	saramaConfig *sarama.Config

	// 重试配置
	maxRetry                 int
	backoff                  retry.BackoffStrategy
	handlerTimeout           time.Duration
	syncRetryMaxTotalTimeout time.Duration
	retryMode                RetryMode
	retryWorkers             int
	retryStore               RetryStore

	// 失败处理
	failedHandler       FailedHandlerFunc
	groupFailedHandlers map[string]FailedHandlerFunc

	// Panic 处理
	panicHandler func(any)

	// 预注册消费者
	consumers []ConsumerRegistration
}

// WithMaxRetry 设置最大重试次数（默认 0，即不重试）
func WithMaxRetry(n int) ConsumerOption {
	return func(c *consumerConfig) {
		c.maxRetry = n
	}
}

// WithBackoff 设置退避策略
func WithBackoff(b retry.BackoffStrategy) ConsumerOption {
	return func(c *consumerConfig) {
		c.backoff = b
	}
}

// WithHandlerTimeout 设置单次 handler 调用的超时时间
func WithHandlerTimeout(d time.Duration) ConsumerOption {
	return func(c *consumerConfig) {
		c.handlerTimeout = d
	}
}

// WithPanicHandler 设置 panic 恢复后的回调函数
func WithPanicHandler(fn func(any)) ConsumerOption {
	return func(c *consumerConfig) {
		c.panicHandler = fn
	}
}

// WithRetryMode 设置重试模式（同步或异步）
func WithRetryMode(mode RetryMode) ConsumerOption {
	return func(c *consumerConfig) {
		c.retryMode = mode
	}
}

// WithRetryWorkers 设置异步重试的 worker 数量（默认 CPU 核数）
func WithRetryWorkers(n int) ConsumerOption {
	return func(c *consumerConfig) {
		c.retryWorkers = n
	}
}

// WithRetryMaxQueueSize 设置内存重试队列的最大容量（仅 MemoryRetryStore 有效）
func WithRetryMaxQueueSize(n int) ConsumerOption {
	return func(c *consumerConfig) {
		c.retryStore = NewMemoryRetryStore(WithMemoryMaxQueueSize(n))
	}
}

// WithSyncRetryMaxTotalTimeout 设置同步重试的最大总超时时间
func WithSyncRetryMaxTotalTimeout(d time.Duration) ConsumerOption {
	return func(c *consumerConfig) {
		c.syncRetryMaxTotalTimeout = d
	}
}

// WithRetryStore 设置异步重试的存储后端
func WithRetryStore(store RetryStore) ConsumerOption {
	return func(c *consumerConfig) {
		c.retryStore = store
	}
}

// WithFailedHandler 设置全局失败处理回调
func WithFailedHandler(fn FailedHandlerFunc) ConsumerOption {
	return func(c *consumerConfig) {
		c.failedHandler = fn
	}
}

// WithConsumeGroupFailedHandler 设置指定消费组的失败处理回调（覆盖全局设置）
func WithConsumeGroupFailedHandler(group string, fn FailedHandlerFunc) ConsumerOption {
	return func(c *consumerConfig) {
		if c.groupFailedHandlers == nil {
			c.groupFailedHandlers = make(map[string]FailedHandlerFunc)
		}
		c.groupFailedHandlers[group] = fn
	}
}

// WithConsumers 批量预注册消费者
func WithConsumers(regs ...ConsumerRegistration) ConsumerOption {
	return func(c *consumerConfig) {
		c.consumers = append(c.consumers, regs...)
	}
}

// WithConsumer 预注册单个消费者
func WithConsumer(group string, handler IHandler, topic string, topics ...string) ConsumerOption {
	return func(c *consumerConfig) {
		allTopics := append([]string{topic}, topics...)
		c.consumers = append(c.consumers, ConsumerRegistration{
			Group:   group,
			Handler: handler,
			Topics:  allTopics,
		})
	}
}

// WithConsumerLogger 设置消费者日志器
func WithConsumerLogger(l *slog.Logger) ConsumerOption {
	return func(c *consumerConfig) {
		c.logger = l
	}
}

// WithConsumerTimeout 设置消费者连接超时时间（默认 5s）
func WithConsumerTimeout(d time.Duration) ConsumerOption {
	return func(c *consumerConfig) {
		c.timeout = d
	}
}

// WithConsumerSaramaConfig 设置自定义 sarama.Config（覆盖默认构建）
func WithConsumerSaramaConfig(cfg *sarama.Config) ConsumerOption {
	return func(c *consumerConfig) {
		c.saramaConfig = cfg
	}
}

// ==================== Producer 选项 ====================

// ProducerOption 生产者配置选项
type ProducerOption func(*producerConfig)

// producerConfig 生产者引擎配置（未导出）
type producerConfig struct {
	logger       *slog.Logger
	timeout      time.Duration
	saramaConfig *sarama.Config
}

// WithProducerTimeout 设置生产者连接超时时间（默认 5s）
func WithProducerTimeout(d time.Duration) ProducerOption {
	return func(c *producerConfig) {
		c.timeout = d
	}
}

// WithProducerLogger 设置生产者日志器
func WithProducerLogger(l *slog.Logger) ProducerOption {
	return func(c *producerConfig) {
		c.logger = l
	}
}

// WithProducerSaramaConfig 设置自定义 sarama.Config（覆盖默认构建）
func WithProducerSaramaConfig(cfg *sarama.Config) ProducerOption {
	return func(c *producerConfig) {
		c.saramaConfig = cfg
	}
}
