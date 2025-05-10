package kafkaconsumer

import (
	"context"

	"github.com/gomooth/pkg/mq/kafkaconsumer/internal"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/save95/xerror"
	"github.com/save95/xlog"
)

type server struct {
	ctx    context.Context
	logger xlog.XLogger

	addrs     []string
	consumers []IConsumer

	panicHandler  func(interface{})
	failedHandler func(group, topic string, msg []byte, err error)

	maxRetry                uint
	useDefaultFailedHandler bool

	consumer internal.IConsumer
}

func NewServer(addrs []string, opts ...func(*server)) IConsumeSever {
	svr := &server{
		addrs:     addrs,
		consumers: make([]IConsumer, 0),
		logger:    logger.NewConsoleLogger(),
	}

	for _, opt := range opts {
		opt(svr)
	}

	return svr
}

func (s *server) Register(group string, handler IHandler, topic string, topics ...string) {
	topics = append([]string{topic}, topics...)
	s.consumers = append(s.consumers, &consumer{
		group:   group,
		topics:  topics,
		handler: handler,
	})
}

func (s *server) Count() uint {
	return uint(len(s.consumers))
}

func (s *server) Start() error {
	if s.consumers == nil || len(s.consumers) == 0 {
		return xerror.New("no register consumer")
	}

	if s.logger == nil {
		s.logger = logger.NewConsoleLogger()
	}
	if s.failedHandler == nil && s.useDefaultFailedHandler {
		s.failedHandler = newDefaultFailedHandler(s.logger).Print
	}

	s.consumer = internal.NewDefaultConsumer(s.ctx, s.addrs)
	s.consumer.SetLogger(s.logger)
	s.consumer.SetMaxRetry(s.maxRetry)
	s.consumer.SetPanicHandler(s.panicHandler)
	s.consumer.SetFailedHandler(s.failedHandler)

	for _, item := range s.consumers {
		if err := s.consumer.RegisterHandler(item.Group(), item.Topics(), item.Handler()); nil != err {
			return err
		}
	}

	s.consumer.Run()
	return nil
}

func (s *server) Shutdown() error {
	if s.consumer != nil {
		_ = s.consumer.Close()
	}

	return nil
}
