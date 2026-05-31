package httpsqs

import (
	"context"

	"github.com/gomooth/pkg/framework/app"
)

// ==================== Consumer ====================

// IHandler HTTPSQS 消息处理器接口
type IHandler interface {
	// Handle 处理消息
	// queue: 队列名，data: 消息内容，pos: 消息在队列中的位置
	Handle(ctx context.Context, queue string, data string, pos int64) error
}

// DeadLetterHandler 可选死信接口，重试耗尽后调用。
type DeadLetterHandler interface {
	OnDeadLetter(ctx context.Context, queue string, data string, pos int64, lastErr error) error
}

// FuncHandler 函数适配器，将函数转换为 IHandler
type FuncHandler func(ctx context.Context, queue string, data string, pos int64) error

func (f FuncHandler) Handle(ctx context.Context, queue string, data string, pos int64) error {
	return f(ctx, queue, data, pos)
}

// IConsumeServer 消费者服务接口（集成 app.IApp 生命周期）
type IConsumeServer interface {
	app.IApp
	app.HealthChecker
	Register(queue string, handler IHandler, opts ...QueueOption)
	Count() uint
}

// ConsumerRegistration 消费者注册信息
type ConsumerRegistration struct {
	Queue   string
	Handler IHandler
	Opts    []QueueOption
}

// ==================== 重试模式 ====================

// RetryMode 重试模式
type RetryMode int

const (
	// RetryModeSync 同步阻塞重试：Handle 失败后在当前 goroutine 中立即重试
	RetryModeSync RetryMode = iota
	// RetryModeRequeue 再入队重试：Handle 失败后通过 HTTPSQS Put 将消息放回队列尾部
	RetryModeRequeue
)

// ==================== 失败处理器 ====================

// FailedHandlerFunc 消费失败处理器（重试耗尽后调用）
type FailedHandlerFunc func(ctx context.Context, queue string, data string, pos int64, err error)

// ==================== 队列级别配置 ====================

// QueueOption 单队列级别配置（覆盖全局默认值）
type QueueOption func(*queueConfig)
