package queue

import (
	"context"

	"github.com/save95/xerror"
)

type server struct {
	ctx       context.Context
	consumers []IConsumer
}

func NewServer(ctx context.Context) IConsumeServer {
	return &server{
		ctx:       ctx,
		consumers: make([]IConsumer, 0),
	}
}

func (s *server) Register(consumer IConsumer) {
	if consumer == nil {
		return
	}

	s.consumers = append(s.consumers, consumer)
}

func (s *server) Count() uint {
	return uint(len(s.consumers))
}

func (s *server) Start() error {
	if s.consumers == nil || len(s.consumers) == 0 {
		return xerror.New("no register consumers")
	}

	for _, consumer := range s.consumers {
		c := consumer
		go func() {
			_ = c.Consume(s.ctx)
		}()
	}

	return nil
}

func (s *server) Shutdown() error {
	//if s.c != nil {
	//	global.Log.Infof("listener server stop")
	//	s.c.Stop()
	//}

	return nil
}
