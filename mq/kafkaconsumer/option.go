package kafkaconsumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/kafkaconsumer/internal"
)

// WithLogger 注册日志处理器
func WithLogger(logger *slog.Logger) func(*server) {
	return func(server *server) {
		server.logger = logger
	}
}

// WithMaxRetry 失败最大重试次数。
// 默认为 0（不重试，由 Kafka 至少一次语义保证重新投递）。
// 注意：RetryModeSync 模式下高重试次数会长时间阻塞分区，
// 建议生产环境使用 RetryModeAsyncWatermark 或 RetryModeAsyncRedis，
// 或配合 WithSyncRetryMaxTotalTimeout 限制总重试时间。
func WithMaxRetry(retry int) func(*server) {
	return func(server *server) {
		server.maxRetry = uint(retry)
	}
}

// WithPanicHandler 消费者 Panic 拦截器
func WithPanicHandler(handler func(any)) func(*server) {
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
func WithConsumeGroupFailedHandler(handler func(ctx context.Context, consumerGroup, topic string, msg []byte, err error)) func(*server) {
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
		server.consumers = append(server.consumers, consumer)
		server.consumers = append(server.consumers, others...)
	}
}

// WithBackoff 设置重试退避策略，默认为 ExponentialDelay{Base: 10s, Max: 5min}
func WithBackoff(backoff retry.BackoffStrategy) func(*server) {
	return func(server *server) {
		server.backoff = backoff
	}
}

// WithRetryMode 设置重试模式，默认为 RetryModeSync（同步阻塞）。
// 可选：
//   - RetryModeSync: 同步阻塞重试（默认）
//   - RetryModeAsyncWatermark: 异步重试 + 水位线 offset 跟踪，不依赖 Redis，重启后 Kafka 重投递
//   - RetryModeAsyncRedis: 异步重试 + Redis 持久化，不丢消息，需提供 RedisRetryStore
func WithRetryMode(mode internal.RetryMode) func(*server) {
	return func(server *server) {
		server.retryMode = mode
	}
}

// WithRetryWorkers 设置异步重试的 worker 协程数，默认为 runtime.NumCPU()。
// 仅对 RetryModeAsyncWatermark 和 RetryModeAsyncRedis 生效。
func WithRetryWorkers(n int) func(*server) {
	return func(server *server) {
		if n > 0 {
			server.retryWorkers = n
		}
	}
}

// WithRetryMaxQueueSize 设置异步重试内存队列的最大容量。
// 仅对 RetryModeAsyncWatermark 生效，0 表示无限制（默认）。
// 当队列满时，新失败的消息将降级为直接走死信/失败回调处理，而非入队重试。
func WithRetryMaxQueueSize(n int) func(*server) {
	return func(server *server) {
		if n > 0 {
			server.retryMaxQueueSize = n
		}
	}
}

// WithRetryRedisStore 设置 Redis 重试状态存储，仅对 RetryModeAsyncRedis 模式生效。
// 可使用 internal.NewDefaultRedisRetryStore(client, consumerGroup) 创建默认实现，
// 也可实现 internal.RedisRetryStore 接口自定义存储。
func WithRetryRedisStore(store internal.RedisRetryStore) func(*server) {
	return func(server *server) {
		server.redisStore = store
	}
}

// WithSyncRetryMaxTotalTimeout 设置同步重试模式下单条消息的总重试超时上限（含退避等待时间）。
// 超时后停止重试，走死信/失败回调处理。0 表示不限（默认）。
// 仅对 RetryModeSync 生效。
func WithSyncRetryMaxTotalTimeout(d time.Duration) func(*server) {
	return func(server *server) {
		server.syncRetryMaxTotalTimeout = d
	}
}

// WithHandlerTimeout 设置单次消息处理的超时时间。
// handler 执行超时视为该次尝试失败，可进入重试流程。
// 0 表示不限（默认）。对三种重试模式统一生效。
func WithHandlerTimeout(d time.Duration) func(*server) {
	return func(server *server) {
		server.handlerTimeout = d
	}
}
