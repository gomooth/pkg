package queue

import (
	"context"

	"github.com/gomooth/pkg/framework/app"
)

// IQueue 简单队列约定
type IQueue interface {
	Push(ctx context.Context, value string) error
	Pop(ctx context.Context) (string, error)
	Close() error
}

// Fetcher 定义从消息源获取一条消息的抽象
// 返回空字符串表示队列为空，返回 error 表示获取失败
type Fetcher interface {
	// Fetch 从消息源获取一条消息
	Fetch(ctx context.Context) (string, error)
	// Close 关闭消息源连接
	Close() error
}

// IConsumer 消费者约定
//
// 生命周期：Consume 阻塞运行消费循环，Close 用于优雅停止。
// Close 应为幂等操作，在 Consume 返回后调用 Close 不应产生副作用。
// 典型调用顺序：Start → Consume(ctx) 阻塞 → 外部调用 Close 或取消 ctx → Consume 返回。
type IConsumer interface {
	Consume(ctx context.Context) error
	Close() error
}

type IRegister interface {
	Register(consumer IConsumer)
	Count() uint
}

type IConsumeServer interface {
	app.IApp
	IRegister
}

// IHandler 消费者处理接口
//
// 生命周期：OnBefore → Handle → (OnFailed)
// OnBefore 为前置钩子，返回错误时触发退避重试
// Handle 为消息处理逻辑，返回错误时调用 OnFailed
type IHandler interface {
	// QueueName 需要处理的队列名
	QueueName() string

	// OnBefore 前置操作，返回错误时触发退避重试
	OnBefore(ctx context.Context) error

	// Handle 消费队列数据
	Handle(ctx context.Context, data string) error

	// OnFailed 消费失败回调
	OnFailed(ctx context.Context, data string, err error)
}

// FuncHandler 将函数适配为 IHandler 接口，适用于简单场景
type FuncHandler struct {
	QueueNameFunc func() string
	HandleFunc    func(ctx context.Context, data string) error
	OnBeforeFunc  func(ctx context.Context) error
	OnFailedFunc  func(ctx context.Context, data string, err error)
}

func (h *FuncHandler) QueueName() string {
	if h.QueueNameFunc != nil {
		return h.QueueNameFunc()
	}
	return ""
}

func (h *FuncHandler) OnBefore(ctx context.Context) error {
	if h.OnBeforeFunc != nil {
		return h.OnBeforeFunc(ctx)
	}
	return nil
}

func (h *FuncHandler) Handle(ctx context.Context, data string) error {
	if h.HandleFunc != nil {
		return h.HandleFunc(ctx, data)
	}
	return nil
}

func (h *FuncHandler) OnFailed(ctx context.Context, data string, err error) {
	if h.OnFailedFunc != nil {
		h.OnFailedFunc(ctx, data, err)
	}
}
