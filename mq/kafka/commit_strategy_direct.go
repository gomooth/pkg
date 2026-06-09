package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/mq/internal/logutil"
)

// directMarkStrategy 直接标记模式的 CommitStrategy 实现。
// 替代 asyncRetryEngine 中 wmStore == nil 的所有分支逻辑。
// Redis 模式下 OnMessage 中成功时直接 MarkMessage，不通过 strategy；
// 重试耗尽后也直接 MarkMessage。
type directMarkStrategy struct {
	store  RetryStore
	logger logutil.Logger
}

// newDirectMarkStrategy 创建直接标记策略实例
func newDirectMarkStrategy(store RetryStore, logger logutil.Logger) *directMarkStrategy {
	return &directMarkStrategy{
		store:  store,
		logger: logger,
	}
}

// OnSuccess 消息处理成功后：无操作（Redis 模式下 OnMessage 中成功时直接 MarkMessage）
func (s *directMarkStrategy) OnSuccess(_ context.Context, _ sarama.ConsumerGroupSession, _ *RetryItem) {
	// Redis 模式：成功时已在 OnMessage 中直接 MarkMessage，无需额外操作
}

// OnExhausted 重试耗尽后：无操作（Redis 模式下 exhausted 后直接 MarkMessage）
func (s *directMarkStrategy) OnExhausted(_ context.Context, _ sarama.ConsumerGroupSession, _ *RetryItem) {
	// Redis 模式：exhausted 后已在 OnMessage 中直接 MarkMessage，无需额外操作
}

// OnScheduleFailed Schedule 失败降级为 exhausted 后：无操作
func (s *directMarkStrategy) OnScheduleFailed(_ context.Context, _ sarama.ConsumerGroupSession, _ *RetryItem) {
	// Redis 模式：Schedule 失败后已在 OnMessage 中直接 MarkMessage，无需额外操作
}

// StartWorkers 启动 redisPollLoop worker 协程
func (s *directMarkStrategy) StartWorkers(ctx context.Context, wg *sync.WaitGroup, processFn func(ctx context.Context, item *RetryItem)) {
	wg.Add(1)
	go s.redisPollLoop(ctx, wg, processFn)
}

// redisPollLoop Redis 模式的轮询 worker
func (s *directMarkStrategy) redisPollLoop(ctx context.Context, wg *sync.WaitGroup, processFn func(ctx context.Context, item *RetryItem)) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			if s.logger != nil {
				s.logger.Error("redisPollLoop panic recovered", "panic", r)
			}
		}
	}()

	const (
		minInterval   = 200 * time.Millisecond
		maxInterval   = 5 * time.Second
		backoffFactor = 2.0
	)

	interval := minInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			items, err := s.store.Fetch(ctx, time.Now(), 10)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("fetch pending retries failed", "error", err)
				}
				continue
			}
			if len(items) > 0 {
				interval = minInterval
				for _, item := range items {
					processFn(ctx, item)
				}
			} else {
				interval = time.Duration(float64(interval) * backoffFactor)
				if interval > maxInterval {
					interval = maxInterval
				}
			}
			ticker.Reset(interval)
		}
	}
}

// OnClearSession session 结束时：无操作
func (s *directMarkStrategy) OnClearSession() {
	// Redis 模式无需重置 partition
}

// OnShutdown 关闭时：无操作
func (s *directMarkStrategy) OnShutdown(_ context.Context) {
	// Redis 模式无需通知 wmStore
}

// 编译时接口检查
var _ CommitStrategy = (*directMarkStrategy)(nil)