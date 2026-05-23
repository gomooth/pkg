package httpsqsconsumer

import (
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
)

// WithLogger 设置日志器
func WithLogger(log *slog.Logger) func(*consumerOptions) {
	return func(o *consumerOptions) {
		o.log = log
	}
}

// WithHandler 设置消费者处理器
func WithHandler(handler IHandler) func(*consumerOptions) {
	return func(o *consumerOptions) {
		o.handler = handler
	}
}

// WithBackoff 设置重试退避策略，默认为 ExponentialDelay{Base: 1min, Max: 24h}
func WithBackoff(backoff retry.BackoffStrategy) func(*consumerOptions) {
	return func(o *consumerOptions) {
		o.backoff = backoff
	}
}

// WithEmptyQueueSleep 设置队列为空时的休眠时间，默认 1 分钟
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
