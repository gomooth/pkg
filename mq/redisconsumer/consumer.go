package redisconsumer

import (
	"context"
	"time"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/mq/queue"

	"github.com/save95/xlog"
)

type consumer struct {
	queueName string
	config    *queue.RedisQueueConfig

	log xlog.XLogger

	fun           func(val string) error
	failedHandler func(val string, err error)
}

func New(opts ...func(*consumer)) queue.IConsumer {
	c := &consumer{
		log: logger.NewConsoleLogger(),
		fun: func(val string) error {
			return nil
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (q *consumer) Consume(ctx context.Context) error {
	queued := queue.NewSimpleRedis(q.config, q.queueName)

	for {
		str, err := queued.Pop(ctx)
		if nil != err {
			q.log.Warningf("get redis queue item failed: [%s]: %+v", q.queueName, err)
			continue
		}

		if len(str) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		if err := q.fun(str); nil != err {
			if q.failedHandler != nil {
				q.failedHandler(str, err)
				continue
			}

			q.log.Warningf("handle redis queue item failed: [%s]: [%s] %+v", q.queueName, str, err)
		}
	}
}
