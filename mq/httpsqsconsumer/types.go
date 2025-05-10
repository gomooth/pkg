package httpsqsconsumer

import (
	"context"

	"github.com/gomooth/httpsqs"
)

// IHandler HTTPSQS 队列消费者处理接口
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
