package queue

import (
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
)

// WithConsumerLogger 设置日志器
func WithConsumerLogger(log *slog.Logger) BaseConsumerOption {
	return func(c *BaseConsumer) {
		c.log = log
	}
}

// WithConsumerBackoff 设置重试退避策略
func WithConsumerBackoff(backoff retry.BackoffStrategy) BaseConsumerOption {
	return func(c *BaseConsumer) {
		c.backoff = backoff
	}
}

// WithConsumerEmptyQueueSleep 设置队列为空时的休眠时间
func WithConsumerEmptyQueueSleep(d time.Duration) BaseConsumerOption {
	return func(c *BaseConsumer) {
		if d > 0 {
			c.emptyQueueSleep = d
		}
	}
}

// WithConsumerFailedCallbackDelay 设置处理失败后回调 OnFailed 的延迟时间
func WithConsumerFailedCallbackDelay(d time.Duration) BaseConsumerOption {
	return func(c *BaseConsumer) {
		if d > 0 {
			c.failedCallbackDelay = d
		}
	}
}

// WithConsumerHandler 设置消费者处理接口
func WithConsumerHandler(handler IHandler) BaseConsumerOption {
	return func(c *BaseConsumer) {
		c.handler = handler
	}
}

// WithConsumerFetcher 设置消息获取器
func WithConsumerFetcher(fetcher Fetcher) BaseConsumerOption {
	return func(c *BaseConsumer) {
		c.fetcher = fetcher
	}
}
