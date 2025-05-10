package redisconsumer

import (
	"github.com/gomooth/pkg/mq/queue"
	"github.com/save95/xlog"
)

// WithLogger 设置日志器
func WithLogger(log xlog.XLogger) func(*consumer) {
	return func(c *consumer) {
		c.log = log
	}
}

// WithHandler 设置消费者处理器
func WithHandler(config *queue.RedisQueueConfig, queueName string, fun func(val string) error) func(*consumer) {
	return func(c *consumer) {
		c.config = config
		c.queueName = queueName
		c.fun = fun
	}
}

// WithFailedHandler 设置消费失败处理器
func WithFailedHandler(handler func(val string, err error)) func(*consumer) {
	return func(c *consumer) {
		c.failedHandler = handler
	}
}
