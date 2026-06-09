package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/mq/internal/logutil"
)

// topicPartition 用于 watermarkStrategy 中 trackedParts map 的 key
type topicPartition struct {
	topic     string
	partition int32
}

// watermarkStrategy 水位线模式的 CommitStrategy 实现。
// 替代 asyncRetryEngine 中 wmStore != nil 的所有分支逻辑。
type watermarkStrategy struct {
	wmStore      WatermarkStore
	trackedParts map[topicPartition]bool
	tpMu         sync.Mutex
	logger       logutil.Logger
}

// newWatermarkStrategy 创建水位线策略实例
func newWatermarkStrategy(wmStore WatermarkStore, logger logutil.Logger) *watermarkStrategy {
	return &watermarkStrategy{
		wmStore:      wmStore,
		trackedParts: make(map[topicPartition]bool),
		logger:       logger,
	}
}

// OnSuccess 消息处理成功后：标记成功 + 提交水位线
func (s *watermarkStrategy) OnSuccess(_ context.Context, session sarama.ConsumerGroupSession, item *RetryItem) {
	s.wmStore.MarkSuccess(item.Topic, item.Partition, item.Offset)
	commitWatermark(session, item.Topic, item.Partition, s.wmStore)
}

// OnExhausted 重试耗尽后：移除 pending + 提交水位线
func (s *watermarkStrategy) OnExhausted(_ context.Context, session sarama.ConsumerGroupSession, item *RetryItem) {
	s.wmStore.RemovePending(item.Topic, item.Partition, item.Offset)
	commitWatermark(session, item.Topic, item.Partition, s.wmStore)
}

// OnScheduleFailed Schedule 失败降级为 exhausted 后：仅提交水位线
func (s *watermarkStrategy) OnScheduleFailed(_ context.Context, session sarama.ConsumerGroupSession, item *RetryItem) {
	commitWatermark(session, item.Topic, item.Partition, s.wmStore)
}

// StartWorkers 启动水位线 worker 协程
func (s *watermarkStrategy) StartWorkers(ctx context.Context, wg *sync.WaitGroup, processFn func(ctx context.Context, item *RetryItem)) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if s.logger != nil {
					s.logger.Error("watermarkWorker panic recovered", "panic", r)
				}
			}
		}()

		notifyCh := s.wmStore.Notify()
		for {
			items, err := s.wmStore.Fetch(ctx, time.Now(), 1)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				continue
			}
			if len(items) > 0 {
				processFn(ctx, items[0])
				continue
			}

			select {
			case <-notifyCh:
			case <-ctx.Done():
				return
			}
		}
	}()
}

// OnClearSession session 结束时：重置所有跟踪的 partition
func (s *watermarkStrategy) OnClearSession() {
	s.tpMu.Lock()
	for tp := range s.trackedParts {
		s.wmStore.ResetPartition(tp.topic, tp.partition)
	}
	s.trackedParts = make(map[topicPartition]bool)
	s.tpMu.Unlock()
}

// OnShutdown 关闭时：通知 wmStore 的等待 goroutine
func (s *watermarkStrategy) OnShutdown(_ context.Context) {
	select {
	case s.wmStore.Notify() <- struct{}{}:
	default:
	}
}

// trackPartition 记录跟踪的 partition（供 asyncRetryEngine 调用）
func (s *watermarkStrategy) trackPartition(topic string, partition int32) {
	s.tpMu.Lock()
	s.trackedParts[topicPartition{topic: topic, partition: partition}] = true
	s.tpMu.Unlock()
}

// commitWatermark 提交水位线以内的 offset（独立函数，接收 wmStore 参数）
func commitWatermark(session sarama.ConsumerGroupSession, topic string, partition int32, wmStore WatermarkStore) {
	wm, ok := wmStore.Watermark(topic, partition)
	if ok {
		session.MarkOffset(topic, partition, wm+1, "")
	}
}

// 编译时接口检查
var _ CommitStrategy = (*watermarkStrategy)(nil)