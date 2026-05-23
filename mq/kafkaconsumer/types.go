package kafkaconsumer

import (
	"context"

	"github.com/gomooth/pkg/framework/app"
)

// IHandler 定义消息处理器接口
type IHandler interface {
	Handle(ctx context.Context, topic string, msg []byte) error
}

// DeadLetterHandler 可选接口，实现此接口的 IHandler 在重试耗尽后将走死信逻辑。
// 未实现此接口的 handler 在重试耗尽后仅调用 failedHandler 打印日志。
type DeadLetterHandler interface {
	OnDeadLetter(ctx context.Context, topic string, msg []byte, err error) error
}

// FuncHandler 将函数类型适配为 IHandler 接口的实现
type FuncHandler func(ctx context.Context, topic string, msg []byte) error

func (fh FuncHandler) Handle(ctx context.Context, topic string, msg []byte) error {
	return fh(ctx, topic, msg)
}

type IRegister interface {
	Register(group string, handler IHandler, topic string, topics ...string)
	Count() uint
}

type IConsumer interface {
	Group() string
	Topics() []string
	Handler() IHandler
	DeadLetterHandler() func(context.Context, string, []byte, error) error
}

type IConsumeServer interface {
	app.IApp
	IRegister
}

type consumer struct {
	group             string
	topics            []string
	handler           IHandler
	deadLetterHandler func(context.Context, string, []byte, error) error
}

func (l *consumer) Group() string {
	return l.group
}

func (l *consumer) Topics() []string {
	return l.topics
}

func (l *consumer) Handler() IHandler {
	return l.handler
}

func (l *consumer) DeadLetterHandler() func(context.Context, string, []byte, error) error {
	return l.deadLetterHandler
}

func NewListener(group string, handler IHandler, topic string, topics ...string) IConsumer {
	topics = append([]string{topic}, topics...)
	return &consumer{
		group:   group,
		topics:  topics,
		handler: handler,
	}
}
