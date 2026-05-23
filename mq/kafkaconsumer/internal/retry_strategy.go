package internal

import (
	"context"

	"github.com/IBM/sarama"
)

// RetryStrategy 抽象消息重试行为，每种 RetryMode 对应一个实现
type RetryStrategy interface {
	// OnMessage 处理每条从 Kafka claim 到达的消息
	OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage)

	// SetSession 在 consumer group session 建立/变更时调用（Setup 阶段）
	SetSession(session sarama.ConsumerGroupSession)

	// ClearSession 在 consumer group session 结束时调用（Cleanup 阶段）
	ClearSession()

	// OnShutdown 优雅关闭，排空重试队列
	OnShutdown(ctx context.Context)

	// SetLogger 注入日志
	SetLogger(l Logger)

	// SetMetrics 注入指标回调
	SetMetrics(m MetricsCallbacks)
}

// Logger 重试策略使用的日志接口，避免依赖具体日志库
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
