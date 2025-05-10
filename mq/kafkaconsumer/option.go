package kafkaconsumer

import (
	"context"

	"github.com/save95/xlog"
)

// WithContext 设置 Context
func WithContext(ctx context.Context) func(*server) {
	return func(server *server) {
		server.ctx = ctx
	}
}

// WithLogger 注册日志处理器
func WithLogger(logger xlog.XLogger) func(*server) {
	return func(server *server) {
		server.logger = logger
	}
}

// WithMaxRetry 失败最大重试次数
func WithMaxRetry(retry int) func(*server) {
	return func(server *server) {
		server.maxRetry = uint(retry)
	}
}

// WithPanicHandler 消费者 Panic 拦截器
func WithPanicHandler(handler func(interface{})) func(*server) {
	return func(server *server) {
		server.panicHandler = handler
	}
}

// WithConsumeGroupDefaultFailedHandler 消费组失败处理器
func WithConsumeGroupDefaultFailedHandler() func(*server) {
	return func(server *server) {
		server.useDefaultFailedHandler = true
	}
}

// WithConsumeGroupFailedHandler 消费组失败处理器
func WithConsumeGroupFailedHandler(handler func(consumerGroup, topic string, msg []byte, err error)) func(*server) {
	return func(server *server) {
		server.failedHandler = handler
	}
}

// WithConsumers 覆写消费者。
// 注意：该操作会直接覆盖已注册的消费者
func WithConsumers(consumers []IConsumer) func(*server) {
	return func(server *server) {
		server.consumers = consumers
	}
}

// WithConsumer 追加消费者
func WithConsumer(consumer IConsumer, others ...IConsumer) func(*server) {
	return func(server *server) {
		if server.consumers == nil {
			server.consumers = make([]IConsumer, 0)
		}
		consumers := append(server.consumers, consumer)
		server.consumers = append(consumers, others...)
	}
}
