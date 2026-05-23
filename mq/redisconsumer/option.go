package redisconsumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/queue"
)

// WithLogger 设置日志器
func WithLogger(log *slog.Logger) func(*consumerOptions) {
	return func(o *consumerOptions) {
		o.log = log
	}
}

// WithHandler 设置消费者处理器
func WithHandler(config *queue.RedisQueueConfig, queueName string, fun func(val string) error) func(*consumerOptions) {
	return func(o *consumerOptions) {
		o.handlerConfig = &handlerConfig{
			config:     config,
			queueName:  queueName,
			handleFunc: func(_ context.Context, data string) error { return fun(data) },
		}
	}
}

// WithFailedHandler 设置消费失败处理器
// 注意：需在 WithHandler 之后调用，否则无效
func WithFailedHandler(handler func(val string, err error)) func(*consumerOptions) {
	return func(o *consumerOptions) {
		if o.handlerConfig == nil {
			o.handlerConfig = &handlerConfig{}
		}
		o.handlerConfig.onFailedFunc = func(_ context.Context, data string, err error) {
			handler(data, err)
		}
	}
}

// WithBackoff 设置重试退避策略，默认为 FixedDelay{Wait: 1s}
func WithBackoff(backoff retry.BackoffStrategy) func(*consumerOptions) {
	return func(o *consumerOptions) {
		o.backoff = backoff
	}
}

// WithEmptyQueueSleep 设置队列为空时的休眠时间，默认 1 秒
func WithEmptyQueueSleep(d time.Duration) func(*consumerOptions) {
	return func(o *consumerOptions) {
		if d > 0 {
			o.emptyQueueSleep = d
		}
	}
}

// WithFailedCallbackDelay 设置处理失败后回调 OnFailed 的延迟时间，默认 3 秒
func WithFailedCallbackDelay(d time.Duration) func(*consumerOptions) {
	return func(o *consumerOptions) {
		if d > 0 {
			o.failedCallbackDelay = d
		}
	}
}
