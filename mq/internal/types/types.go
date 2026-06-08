package types

import "context"

// IHandler 统一消息处理接口。
// 各 MQ 实现的 handler 均适配为此接口，支持跨 MQ 通用 handler。
type IHandler interface {
	Handle(ctx context.Context, msg Message) error
}

// FailedHandlerFunc 统一失败处理回调函数类型。
// 重试耗尽后调用，msg 携带原始消息信息和 MQ 专有字段。
type FailedHandlerFunc func(ctx context.Context, msg Message, err error)

// DeadLetterHandler 可选死信接口，重试耗尽后调用。
// 若 handler 实现了此接口，优先于 FailedHandlerFunc。
type DeadLetterHandler interface {
	OnDeadLetter(ctx context.Context, msg Message, lastErr error) error
}

// FuncHandler 函数适配器，将函数转换为 IHandler
type FuncHandler func(ctx context.Context, msg Message) error

func (f FuncHandler) Handle(ctx context.Context, msg Message) error {
	return f(ctx, msg)
}

// IConsumeServer 统一消费服务接口。
// Register 返回 error 以替代旧版 void 签名。
type IConsumeServer interface {
	Register(dest string, handler IHandler, opts ...RegisterOption) error
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	HealthCheck(ctx context.Context) error
	Count() uint
}

// IProducer 统一生产者接口。
// 原 kafka.IProducer.ProduceOrdered 合并为 Produce + WithOrderKey。
// 包含生命周期方法，与 app.IApp 兼容。
type IProducer interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Produce(ctx context.Context, dest string, message []byte, opts ...ProduceOption) error
	ProduceBatch(ctx context.Context, dest string, messages [][]byte, opts ...ProduceOption) error
}

