package queue

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/xerror"
)

// BaseConsumerOption BaseConsumer 配置选项
type BaseConsumerOption func(*BaseConsumer)

// BaseConsumer 提供通用的消费循环、退避策略和 Close 机制
// 子包通过嵌入 BaseConsumer 并设置 handler/fetcher 来复用消费循环
type BaseConsumer struct {
	handler IHandler
	fetcher Fetcher
	log     *slog.Logger
	backoff retry.BackoffStrategy

	emptyQueueSleep     time.Duration
	failedCallbackDelay time.Duration

	mu       sync.Mutex
	cancel   context.CancelFunc
	failedWg sync.WaitGroup // 跟踪 OnFailed goroutine，确保 Close 时等待完成
}

// NewBaseConsumer 创建 BaseConsumer 实例
func NewBaseConsumer(opts ...BaseConsumerOption) *BaseConsumer {
	c := &BaseConsumer{
		log:                 logger.NewConsoleLogger(),
		backoff:             &retry.ExponentialDelay{Base: time.Minute, Max: 24 * time.Hour},
		emptyQueueSleep:     3 * time.Second,
		failedCallbackDelay: 3 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *BaseConsumer) Consume(ctx context.Context) error {
	if c.handler == nil {
		return xerror.New("no consumer handler")
	}
	if c.fetcher == nil {
		return xerror.New("no consumer fetcher")
	}

	c.mu.Lock()
	ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	queueName := c.handler.QueueName()
	c.log.Debug("consumer start",
		slog.String("queue", queueName),
	)
	defer func() {
		if c.fetcher != nil {
			if err := c.fetcher.Close(); err != nil {
				c.log.Warn("close consumer fetcher failed",
					slog.String("queue", queueName),
					slog.Any("error", err),
				)
			}
		}
		c.log.Debug("consumer end",
			slog.String("queue", queueName),
		)
	}()

	var infraAttempt uint
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.handler.OnBefore(ctx); err != nil {
			delay := c.backoff.Delay(infraAttempt)
			infraAttempt++
			c.log.Error("consumer onBefore failed",
				slog.String("queue", queueName),
				slog.Duration("sleep", delay),
				slog.Uint64("attempt", uint64(infraAttempt)),
				slog.Any("error", err),
			)
			sleepAndWait(ctx, delay)
			continue
		}

		data, err := c.fetcher.Fetch(ctx)
		if err != nil {
			delay := c.backoff.Delay(infraAttempt)
			infraAttempt++
			c.log.Error("consumer fetch failed",
				slog.String("queue", queueName),
				slog.Duration("sleep", delay),
				slog.Uint64("attempt", uint64(infraAttempt)),
				slog.Any("error", err),
			)
			sleepAndWait(ctx, delay)
			continue
		}

		// 队列为空
		if len(data) == 0 {
			infraAttempt = 0 // 队列为空说明连接正常，重置退避计数
			sleepAndWait(ctx, c.emptyQueueSleep)
			continue
		}

		// 成功获取消息，重置基础设施退避计数
		infraAttempt = 0

		// 处理数据
		if err := c.handler.Handle(ctx, data); err != nil {
			c.failedWg.Add(1)
			go func() {
				defer c.failedWg.Done()
				select {
				case <-ctx.Done():
					return
				case <-time.After(c.failedCallbackDelay):
					c.handler.OnFailed(ctx, data, err)
				}
			}()
		}
	}
}

// Close 关闭消费者，取消 Consume 循环并等待 OnFailed goroutine 完成
func (c *BaseConsumer) Close() error {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()
	c.failedWg.Wait()
	return nil
}

// sleepAndWait 按策略延迟等待，支持 context 取消
func sleepAndWait(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
