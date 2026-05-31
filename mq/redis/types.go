package redis

import (
	"context"

	"github.com/gomooth/pkg/framework/app"
)

// ==================== Consumer ====================

// IHandler 消息处理器接口
type IHandler interface {
	Handle(ctx context.Context, queue string, message []byte) error
}

// DeadLetterHandler 可选死信接口，重试耗尽后调用。
type DeadLetterHandler interface {
	OnDeadLetter(ctx context.Context, queue string, message []byte, lastErr error) error
}

// FuncHandler 函数适配器，将函数转换为 IHandler
type FuncHandler func(ctx context.Context, queue string, message []byte) error

func (f FuncHandler) Handle(ctx context.Context, queue string, message []byte) error {
	return f(ctx, queue, message)
}

// IConsumeServer 消费者服务接口（集成 app.IApp 生命周期）
type IConsumeServer interface {
	app.IApp
	app.HealthChecker
	Register(queue string, handler IHandler, queues ...string)
	Count() uint
}

// ConsumerRegistration 消费者注册信息
type ConsumerRegistration struct {
	Queue   string
	Handler IHandler
}

// ==================== Producer ====================

// IProducer 生产者接口（集成 app.IApp 生命周期）
type IProducer interface {
	app.IApp
	Produce(ctx context.Context, queue string, message []byte) error
	ProduceBatch(ctx context.Context, queue string, messages ...[]byte) error
}

// ==================== 重试模式 ====================

// RetryMode 重试模式
type RetryMode int

const (
	// RetryModeSync 同步阻塞重试：Handle 失败后在当前循环中立即重试
	RetryModeSync RetryMode = iota
	// RetryModeRequeue 再入队重试：Handle 失败后将消息重新 Push 回队列尾部
	RetryModeRequeue
)

// ==================== 失败处理器 ====================

// FailedHandlerFunc 消费失败处理器（重试耗尽后调用）
type FailedHandlerFunc func(ctx context.Context, queue string, message []byte, err error)
