package redisconsumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/queue"
)

// redisFetcher 从 Redis 队列获取消息，实现 queue.Fetcher 接口
type redisFetcher struct {
	queue queue.IQueue
}

func (f *redisFetcher) Fetch(ctx context.Context) (string, error) {
	return f.queue.Pop(ctx)
}

func (f *redisFetcher) Close() error {
	return f.queue.Close()
}

// consumer Redis 消费者，嵌入 BaseConsumer 复用消费循环
type consumer struct {
	*queue.BaseConsumer
}

// New 创建 Redis 消费者
func New(opts ...func(*consumerOptions)) queue.IConsumer {
	o := defaultConsumerOptions()
	for _, opt := range opts {
		opt(o)
	}

	baseOpts := []queue.BaseConsumerOption{
		queue.WithConsumerLogger(o.log),
		queue.WithConsumerBackoff(o.backoff),
		queue.WithConsumerEmptyQueueSleep(o.emptyQueueSleep),
		queue.WithConsumerFailedCallbackDelay(o.failedCallbackDelay),
	}

	if o.handlerConfig != nil {
		q := queue.NewSimpleRedis(o.handlerConfig.config, o.handlerConfig.queueName)
		fetcher := &redisFetcher{queue: q}
		handler := &queue.FuncHandler{
			QueueNameFunc: func() string { return o.handlerConfig.queueName },
			HandleFunc:    o.handlerConfig.handleFunc,
			OnFailedFunc:  o.handlerConfig.onFailedFunc,
		}
		baseOpts = append(baseOpts,
			queue.WithConsumerHandler(handler),
			queue.WithConsumerFetcher(fetcher),
		)
	}

	return &consumer{
		BaseConsumer: queue.NewBaseConsumer(baseOpts...),
	}
}

type consumerOptions struct {
	log                 *slog.Logger
	backoff             retry.BackoffStrategy
	emptyQueueSleep     time.Duration
	failedCallbackDelay time.Duration
	handlerConfig       *handlerConfig
}

type handlerConfig struct {
	config       *queue.RedisQueueConfig
	queueName    string
	handleFunc   func(ctx context.Context, data string) error
	onFailedFunc func(ctx context.Context, data string, err error)
}

func defaultConsumerOptions() *consumerOptions {
	return &consumerOptions{
		log:                 logger.NewConsoleLogger(),
		backoff:             &retry.FixedDelay{Wait: time.Second},
		emptyQueueSleep:     time.Second,
		failedCallbackDelay: 3 * time.Second,
	}
}
