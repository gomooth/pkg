package httpsqsconsumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/queue"
)

// httpsqsFetcher 从 HTTPSQS 获取消息，实现 queue.Fetcher 接口
type httpsqsFetcher struct {
	client    httpsqs.IClient
	queueName string

	mu      sync.Mutex
	lastPos int64
}

// LastPos 返回最后一次 Fetch 获取的消息位置
func (f *httpsqsFetcher) LastPos() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPos
}

func (f *httpsqsFetcher) Fetch(ctx context.Context) (string, error) {
	str, pos, err := f.client.Get(ctx, f.queueName)
	if err != nil {
		return "", err
	}
	if pos == -1 {
		return "", nil
	}
	f.mu.Lock()
	f.lastPos = pos
	f.mu.Unlock()
	return str, nil
}

func (f *httpsqsFetcher) Close() error { return nil }

// consumer HTTPSQS 消费者，嵌入 BaseConsumer 复用消费循环
type consumer struct {
	*queue.BaseConsumer
}

// New 创建 HTTPSQS 消费者
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

	// 如果设置了 IHandler，构建 httpsqsHandler 适配器和 httpsqsFetcher
	if o.handler != nil {
		client, err := o.handler.GetClient()
		if err == nil {
			fetcher := &httpsqsFetcher{
				client:    client,
				queueName: o.handler.QueueName(),
			}
			handler := &httpsqsHandler{
				inner:   o.handler,
				fetcher: fetcher,
			}
			baseOpts = append(baseOpts,
				queue.WithConsumerHandler(handler),
				queue.WithConsumerFetcher(fetcher),
			)
		}
	}

	return &consumer{
		BaseConsumer: queue.NewBaseConsumer(baseOpts...),
	}
}

// consumerOptions HTTPSQS 消费者配置
type consumerOptions struct {
	log                 *slog.Logger
	handler             IHandler
	backoff             retry.BackoffStrategy
	emptyQueueSleep     time.Duration
	failedCallbackDelay time.Duration
}

func defaultConsumerOptions() *consumerOptions {
	return &consumerOptions{
		log:                 logger.NewConsoleLogger(),
		backoff:             &retry.ExponentialDelay{Base: time.Minute, Max: 24 * time.Hour},
		emptyQueueSleep:     time.Minute,
		failedCallbackDelay: 3 * time.Second,
	}
}
