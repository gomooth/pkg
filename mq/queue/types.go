package queue

import (
	"context"

	"github.com/gomooth/pkg/framework/app"
)

// IQueue 简单队列约定
type IQueue interface {
	Push(ctx context.Context, value string) error
	Pop(ctx context.Context) (string, error)
}

// IConsumer 消费者约定
type IConsumer interface {
	Consume(ctx context.Context) error
}

type IRegister interface {
	Register(consumer IConsumer)
	Count() uint
}

type IConsumeServer interface {
	app.IApp
	IRegister
}
