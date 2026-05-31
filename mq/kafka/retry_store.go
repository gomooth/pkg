package kafka

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gomooth/pkg/mq/kafka/internal"
)

// 编译时接口检查
var (
	_ RetryStore     = (*MemoryRetryStore)(nil)
	_ WatermarkStore = (*MemoryRetryStore)(nil)
)

// ErrRetryQueueFull 当重试队列已满时返回此错误
var ErrRetryQueueFull = retryQueueFullError{}

type retryQueueFullError struct{}

func (retryQueueFullError) Error() string { return "retry queue is full" }

// MemoryStoreOption 内存重试存储配置选项
type MemoryStoreOption func(*memoryStoreConfig)

type memoryStoreConfig struct {
	maxQueueSize int
	logger       internal.Logger
}

// WithMemoryMaxQueueSize 设置最大队列容量（默认 10000）
func WithMemoryMaxQueueSize(n int) MemoryStoreOption {
	return func(c *memoryStoreConfig) {
		c.maxQueueSize = n
	}
}

// WithMemoryLogger 设置自定义日志器
func WithMemoryLogger(logger internal.Logger) MemoryStoreOption {
	return func(c *memoryStoreConfig) {
		c.logger = logger
	}
}

// MemoryRetryStore 基于内存的重试存储，同时实现 RetryStore 和 WatermarkStore 接口。
// 使用优先队列（最小堆）按 NextRetryAt 排序，水位线跟踪器管理 offset 提交。
type MemoryRetryStore struct {
	tracker *internal.WatermarkTracker
	pq      *internal.RetryHeap
	pqMu    sync.Mutex
	notify  chan struct{}
	logger  internal.Logger
}

// NewMemoryRetryStore 创建内存重试存储实例
func NewMemoryRetryStore(opts ...MemoryStoreOption) *MemoryRetryStore {
	cfg := memoryStoreConfig{
		maxQueueSize: 10000,
		logger:       internal.NewSlogLogger(slog.Default()),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	tracker := internal.NewWatermarkTracker(cfg.logger)

	return &MemoryRetryStore{
		tracker: tracker,
		pq:      internal.NewRetryHeap(cfg.maxQueueSize),
		notify:  make(chan struct{}, 1),
		logger:  cfg.logger,
	}
}

// Schedule 将消息加入重试队列
func (s *MemoryRetryStore) Schedule(_ context.Context, item *RetryItem) error {
	internalItem := toInternalRetryItem(item)

	s.pqMu.Lock()
	defer s.pqMu.Unlock()

	if s.pq.IsFull() {
		return ErrRetryQueueFull
	}

	if !s.tracker.MarkPending(item.Topic, item.Partition, item.Offset) {
		s.logger.Warn("watermark pending set overflow, skipping schedule",
			"topic", item.Topic,
			"partition", item.Partition,
			"offset", item.Offset,
		)
		return ErrRetryQueueFull
	}

	s.pq.PushItem(internalItem)
	s.signalNotify()
	return nil
}

// Fetch 获取已到期的重试消息（最多 limit 条）
func (s *MemoryRetryStore) Fetch(_ context.Context, now time.Time, limit int) ([]*RetryItem, error) {
	s.pqMu.Lock()
	defer s.pqMu.Unlock()

	var items []*internal.RetryItem
	for i := 0; i < limit; i++ {
		peek := s.pq.Peek()
		if peek == nil || peek.NextRetryAt.After(now) {
			break
		}
		items = append(items, s.pq.PopItem())
	}

	return toPublicRetryItems(items), nil
}

// Remove 从重试队列中移除消息，并清除其 pending 状态
func (s *MemoryRetryStore) Remove(_ context.Context, item *RetryItem) error {
	s.pqMu.Lock()
	s.pq.Remove(item.Topic, item.Partition, item.Offset)
	s.pqMu.Unlock()

	s.tracker.RemovePending(item.Topic, item.Partition, item.Offset)
	return nil
}

// Reschedule 将旧的重试项替换为新的重试项
func (s *MemoryRetryStore) Reschedule(_ context.Context, oldItem, newItem *RetryItem) error {
	// 移除旧的 pending 状态
	s.tracker.RemovePending(oldItem.Topic, oldItem.Partition, oldItem.Offset)

	internalItem := toInternalRetryItem(newItem)

	s.pqMu.Lock()
	defer s.pqMu.Unlock()

	// 从堆中移除旧项
	s.pq.Remove(oldItem.Topic, oldItem.Partition, oldItem.Offset)

	// 标记新的 pending 状态
	s.tracker.MarkPending(newItem.Topic, newItem.Partition, newItem.Offset)

	// 推入新项
	s.pq.PushItem(internalItem)
	s.signalNotify()
	return nil
}

// LoadAll 内存模式不需要恢复，始终返回 nil
func (s *MemoryRetryStore) LoadAll(_ context.Context) ([]*RetryItem, error) {
	return nil, nil
}

// Close 关闭存储（内存模式无需清理）
func (s *MemoryRetryStore) Close() error {
	return nil
}

// MarkSuccess 标记 offset 已成功处理，推进水位线
func (s *MemoryRetryStore) MarkSuccess(topic string, partition int32, offset int64) {
	s.tracker.MarkSuccess(topic, partition, offset)
}

// RemovePending 移除 pending 状态（重试耗尽后走死信/失败处理）
func (s *MemoryRetryStore) RemovePending(topic string, partition int32, offset int64) {
	s.tracker.RemovePending(topic, partition, offset)
}

// Watermark 返回水位线 offset
func (s *MemoryRetryStore) Watermark(topic string, partition int32) (int64, bool) {
	return s.tracker.Watermark(topic, partition)
}

// ResetPartition 重置某个 partition 的跟踪状态
func (s *MemoryRetryStore) ResetPartition(topic string, partition int32) {
	s.tracker.ResetPartition(topic, partition)
}

// Notify 返回通知通道，当有新的重试项加入时发送信号
func (s *MemoryRetryStore) Notify() chan struct{} {
	return s.notify
}

// signalNotify 非阻塞地发送通知信号
func (s *MemoryRetryStore) signalNotify() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}
