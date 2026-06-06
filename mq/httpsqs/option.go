package httpsqs

import (
	"log/slog"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/types"
)

// ==================== Consumer 选项 ====================

// ConsumerOption 消费者配置选项
type ConsumerOption func(*consumerConfig)

// consumerConfig 消费者引擎配置（未导出）
type consumerConfig struct {
	logger          *slog.Logger
	client          httpsqs.IClient // 全局 HTTPSQS 客户端（必填）
	maxRetry        int
	backoff         retry.BackoffStrategy
	retryMode       types.RetryMode
	handlerTimeout  time.Duration
	emptyQueueSleep time.Duration
	failedHandler   types.FailedHandlerFunc
	panicHandler    func(any)
	consumers       []ConsumerRegistration
}

// WithHTTPSQSClient 设置全局 HTTPSQS 客户端（必填，Start 时校验）
func WithHTTPSQSClient(client httpsqs.IClient) ConsumerOption {
	return func(c *consumerConfig) {
		c.client = client
	}
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

// WithRetryMode 设置重试模式（默认 RetryModeSync）
func WithRetryMode(mode types.RetryMode) ConsumerOption {
	return func(c *consumerConfig) {
		c.retryMode = mode
	}
}

// WithHandlerTimeout 设置单次 handler 调用的超时时间（默认 0，不限）
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
func WithConsumer(queue string, handler types.IHandler, opts ...types.QueueOption) ConsumerOption {
	return func(c *consumerConfig) {
		c.consumers = append(c.consumers, ConsumerRegistration{
			Queue:   queue,
			Handler: handler,
			Opts:    opts,
		})
	}
}

// WithConsumerLogger 设置消费者日志器
func WithConsumerLogger(l *slog.Logger) ConsumerOption {
	return func(c *consumerConfig) {
		c.logger = l
	}
}