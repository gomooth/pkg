package kafkaconsumer

import (
	"github.com/gomooth/pkg/framework/app"
)

type IHandler func(topic string, msg []byte) error

type IRegister interface {
	Register(group string, handler IHandler, topic string, topics ...string)
	Count() uint
}

type IConsumer interface {
	Group() string
	Topics() []string
	Handler() IHandler
}

type IConsumeSever interface {
	app.IApp
	IRegister
}

type consumer struct {
	group   string
	topics  []string
	handler IHandler
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

func NewListener(group string, handler IHandler, topic string, topics ...string) IConsumer {
	topics = append([]string{topic}, topics...)
	return &consumer{
		group:   group,
		topics:  topics,
		handler: handler,
	}
}
