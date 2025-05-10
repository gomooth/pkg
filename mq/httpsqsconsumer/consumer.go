package httpsqsconsumer

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/mq/queue"

	"github.com/save95/xerror"
	"github.com/save95/xlog"
)

type consumer struct {
	log xlog.XLogger

	handler  IHandler
	maxRetry uint
}

func New(opts ...func(consumer *consumer)) queue.IConsumer {
	c := &consumer{
		log: logger.NewConsoleLogger(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (s *consumer) Consume(ctx context.Context) error {
	if s.handler == nil {
		return xerror.New("no httpsqs handler")
	}

	client, err := s.handler.GetClient()
	if nil != err {
		return xerror.Wrap(err, "get httpsqs client failed")
	}

	queueName := s.handler.QueueName()
	s.log.Debugf("[httpsqs] %s consumer, start", queueName)
	defer func() {
		s.log.Debugf("[httpsqs] %s consumer, end", queueName)
	}()

	for {
		if err := s.handler.OnBefore(ctx); nil != err {
			sleep := 2 << s.maxRetry
			s.maxRetry++
			s.log.Errorf("[httpsqs] %s onBefore failed, sleep %d minute: %+v", queueName, sleep, err)
			time.Sleep(time.Duration(sleep) * time.Minute)
			continue
		}

		str, pos, err := client.Get(ctx, queueName)
		if nil != err {
			s.log.Errorf("[httpsqs] %s get queue item failed: %+v", queueName, err)
			continue
		}
		// 消费完则跳过
		if pos == -1 {
			time.Sleep(time.Minute)
			continue
		}
		// 空数据
		if len(str) == 0 {
			time.Sleep(3 * time.Second)
			continue
		}

		// 处理数据
		if err := s.handler.Handle(ctx, str, pos); nil != err {
			go func() {
				time.Sleep(3 * time.Second)

				s.handler.OnFailed(ctx, str, err)
			}()
		}
	}
}
