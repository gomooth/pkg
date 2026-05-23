package httpsqsconsumer

import (
	"context"

	"github.com/gomooth/httpsqs"
)

// IHandler HTTPSQS 队列消费者处理接口（兼容旧代码）
//
// 推荐使用 queue.IHandler 替代。本接口保留以兼容需要 pos 参数的场景。
type IHandler interface {
	// QueueName 需要处理的队列名
	QueueName() string

	// GetClient 获取 HTTPSQS 客户端
	GetClient() (httpsqs.IClient, error)

	// OnBefore 前置操作
	OnBefore(ctx context.Context) error

	// Handle 消费队列数据
	Handle(ctx context.Context, data string, pos int64) error

	// OnFailed 失败回调
	OnFailed(ctx context.Context, data string, err error)
}

// httpsqsHandler 适配器，将 IHandler 桥接到 queue.IHandler
// 通过共享 httpsqsFetcher 缓存的 lastPos 来获取 pos 参数
type httpsqsHandler struct {
	inner   IHandler
	fetcher *httpsqsFetcher
}

func (h *httpsqsHandler) QueueName() string {
	return h.inner.QueueName()
}

func (h *httpsqsHandler) OnBefore(ctx context.Context) error {
	return h.inner.OnBefore(ctx)
}

func (h *httpsqsHandler) Handle(ctx context.Context, data string) error {
	pos := h.fetcher.LastPos()
	return h.inner.Handle(ctx, data, pos)
}

func (h *httpsqsHandler) OnFailed(ctx context.Context, data string, err error) {
	h.inner.OnFailed(ctx, data, err)
}
