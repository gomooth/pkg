package kafka

import (
	"context"
	"sync"

	"github.com/IBM/sarama"
)

// CommitStrategy 封装消息确认和 offset 提交策略。
// 不同的策略实现对应不同的 commit 模式（水位线 / 直接标记），
// 消除 asyncRetryEngine 中的 wmStore 分叉检查。
type CommitStrategy interface {
	// OnSuccess 消息处理成功后的提交逻辑
	OnSuccess(ctx context.Context, session sarama.ConsumerGroupSession, item *RetryItem)

	// OnExhausted 重试耗尽后的提交逻辑
	OnExhausted(ctx context.Context, session sarama.ConsumerGroupSession, item *RetryItem)

	// OnScheduleFailed Schedule 失败降级为 exhausted 后的提交逻辑
	OnScheduleFailed(ctx context.Context, session sarama.ConsumerGroupSession, item *RetryItem)

	// StartWorkers 启动 worker 协程
	StartWorkers(ctx context.Context, wg *sync.WaitGroup, processFn func(ctx context.Context, item *RetryItem))

	// OnClearSession session 结束时的清理逻辑
	OnClearSession()

	// OnShutdown 关闭时的通知逻辑
	OnShutdown(shutdownCtx context.Context)

	// MarkImmediate 消息首次成功时是否立即 MarkMessage。
	// watermarkStrategy: no-op（通过水位线批量提交）
	// directMarkStrategy: session.MarkMessage(msg, "")
	MarkImmediate(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage)

	// OnEnqueue 消息首次放入重试队列时的附加动作。
	// watermarkStrategy: trackPartition(msg.Topic, msg.Partition)
	// directMarkStrategy: no-op
	OnEnqueue(msg *sarama.ConsumerMessage)
}