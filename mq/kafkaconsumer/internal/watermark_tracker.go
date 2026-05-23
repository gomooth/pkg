package internal

import (
	"hash/fnv"
	"log/slog"
	"sync"
)

type topicPartition struct {
	Topic     string
	Partition int32
}

const shardCount = 16

// maxPendingPerPartition 单个 partition 的 pending 集合最大容量。
// 超过此上限后 MarkPending 返回 false，调用方应降级处理。
const maxPendingPerPartition = 10000

type shardData struct {
	mu            sync.RWMutex
	offsets       map[topicPartition]int64
	pending       map[topicPartition]map[int64]struct{}
	watermarks    map[topicPartition]int64
	wmReady       map[topicPartition]bool
	minPending    map[topicPartition]int64 // 每个 (topic, partition) 的最小 pending offset 缓存
	minPendingSet map[topicPartition]int   // 每个 (topic, partition) 的 pending 集合大小（用于判断是否需要重新扫描）
}

// WatermarkTracker 跟踪每个 (topic, partition) 的水位线 offset。
// 水位线 = 所有 <= 水位线的 offset 均已成功处理（无 gap）的最高 offset。
// 用于 Plan B：只提交水位线以内的 offset，保证不跳过未处理的消息。
type WatermarkTracker struct {
	shards [shardCount]shardData
}

func NewWatermarkTracker() *WatermarkTracker {
	t := &WatermarkTracker{}
	for i := range t.shards {
		t.shards[i].offsets = make(map[topicPartition]int64)
		t.shards[i].pending = make(map[topicPartition]map[int64]struct{})
		t.shards[i].watermarks = make(map[topicPartition]int64)
		t.shards[i].wmReady = make(map[topicPartition]bool)
		t.shards[i].minPending = make(map[topicPartition]int64)
		t.shards[i].minPendingSet = make(map[topicPartition]int)
	}
	return t
}

func (t *WatermarkTracker) getShard(tp topicPartition) *shardData {
	h := fnv.New32a()
	h.Write([]byte(tp.Topic))
	h.Write([]byte{byte(tp.Partition >> 24), byte(tp.Partition >> 16), byte(tp.Partition >> 8), byte(tp.Partition)})
	return &t.shards[h.Sum32()%shardCount]
}

// MarkSuccess 标记 offset 已成功处理，推进水位线
func (t *WatermarkTracker) MarkSuccess(topic string, partition int32, offset int64) {
	tp := topicPartition{Topic: topic, Partition: partition}
	s := t.getShard(tp)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 从 pending 中移除
	if p, ok := s.pending[tp]; ok {
		delete(p, offset)
		if len(p) == 0 {
			delete(s.pending, tp)
			delete(s.minPending, tp)
			delete(s.minPendingSet, tp)
		} else if offset == s.minPending[tp] {
			// 移除的是最小 pending offset，需要重新扫描
			s.invalidateMinPending(tp)
		}
	}

	// 更新最高 offset
	if offset > s.offsets[tp] {
		s.offsets[tp] = offset
	}

	// 重新计算水位线
	t.recomputeWatermark(s, tp)
}

// MarkPending 标记 offset 正在重试（尚未成功）。
// 返回 false 表示该 partition 的 pending 集合已达上限，调用方应降级处理。
func (t *WatermarkTracker) MarkPending(topic string, partition int32, offset int64) bool {
	tp := topicPartition{Topic: topic, Partition: partition}
	s := t.getShard(tp)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pending[tp] == nil {
		s.pending[tp] = make(map[int64]struct{})
	}

	// 超限检查
	if len(s.pending[tp]) >= maxPendingPerPartition {
		slog.Warn("watermark: pending set overflow, degrading",
			slog.String("topic", topic),
			slog.Int("partition", int(partition)),
			slog.Int("pending_count", len(s.pending[tp])),
			slog.Int("max_pending", maxPendingPerPartition),
		)
		return false
	}

	s.pending[tp][offset] = struct{}{}

	// 更新最小 pending offset 缓存
	curMin, hasMin := s.minPending[tp]
	if !hasMin || offset < curMin {
		s.minPending[tp] = offset
	}
	s.minPendingSet[tp] = len(s.pending[tp])

	// 更新最高 offset
	if offset > s.offsets[tp] {
		s.offsets[tp] = offset
	}

	// 水位线可能需要回退
	t.recomputeWatermark(s, tp)

	return true
}

// RemovePending 移除 pending 状态（重试耗尽后走死信/失败处理）
func (t *WatermarkTracker) RemovePending(topic string, partition int32, offset int64) {
	tp := topicPartition{Topic: topic, Partition: partition}
	s := t.getShard(tp)

	s.mu.Lock()
	defer s.mu.Unlock()

	if p, ok := s.pending[tp]; ok {
		delete(p, offset)
		if len(p) == 0 {
			delete(s.pending, tp)
			delete(s.minPending, tp)
			delete(s.minPendingSet, tp)
		} else if offset == s.minPending[tp] {
			s.invalidateMinPending(tp)
		}
	}

	// 水位线可能推进
	t.recomputeWatermark(s, tp)
}

// Watermark 返回水位线 offset，即所有 <= 此值的 offset 均已处理完成。
// 返回 (offset, true) 表示存在有效水位线，(0, false) 表示尚无。
func (t *WatermarkTracker) Watermark(topic string, partition int32) (int64, bool) {
	tp := topicPartition{Topic: topic, Partition: partition}
	s := t.getShard(tp)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.wmReady[tp] {
		return 0, false
	}

	wm := s.watermarks[tp]
	if wm < 0 {
		return 0, false
	}
	return wm, true
}

// ResetPartition 重置某个 partition 的跟踪状态（rebalance 后该 partition 不再分配给本消费者）
func (t *WatermarkTracker) ResetPartition(topic string, partition int32) {
	tp := topicPartition{Topic: topic, Partition: partition}
	s := t.getShard(tp)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.offsets, tp)
	delete(s.pending, tp)
	delete(s.watermarks, tp)
	delete(s.wmReady, tp)
	delete(s.minPending, tp)
	delete(s.minPendingSet, tp)
}

// recomputeWatermark 重新计算水位线。
// 优先使用缓存的 minPendingOffset（O(1)），仅在缓存失效时扫描 pending 集合。
// 必须在持有锁的情况下调用。
func (t *WatermarkTracker) recomputeWatermark(s *shardData, tp topicPartition) {
	highest, exists := s.offsets[tp]
	if !exists {
		delete(s.watermarks, tp)
		delete(s.wmReady, tp)
		return
	}

	pendingSet := s.pending[tp]

	if len(pendingSet) == 0 {
		s.watermarks[tp] = highest
		s.wmReady[tp] = true
		return
	}

	// 使用缓存的 minPendingOffset（若有效）
	minPending := s.getCachedMinPending(tp)
	if minPending < 0 || minPending > highest {
		// 所有 pending offset 都 > highest，水位线 = highest
		s.watermarks[tp] = highest
		s.wmReady[tp] = true
		return
	}

	// 水位线 = 最小 pending offset - 1
	wm := minPending - 1
	if wm < 0 {
		wm = -1
	}
	s.watermarks[tp] = wm
	s.wmReady[tp] = true
}

// getCachedMinPending 获取缓存的最小 pending offset。
// 若缓存无效（集合大小变化），则重新扫描并更新缓存。
func (s *shardData) getCachedMinPending(tp topicPartition) int64 {
	pendingSet := s.pending[tp]
	if len(pendingSet) == 0 {
		return -1
	}

	// 缓存有效：集合大小未变化
	if cached, ok := s.minPending[tp]; ok && s.minPendingSet[tp] == len(pendingSet) {
		return cached
	}

	// 缓存失效，重新扫描
	minPending := int64(-1)
	for offset := range pendingSet {
		if minPending < 0 || offset < minPending {
			minPending = offset
		}
	}
	s.minPending[tp] = minPending
	s.minPendingSet[tp] = len(pendingSet)
	return minPending
}

// invalidateMinPending 使最小 pending offset 缓存失效。
func (s *shardData) invalidateMinPending(tp topicPartition) {
	delete(s.minPending, tp)
	delete(s.minPendingSet, tp)
}

// PendingCount 返回指定 (topic, partition) 的 pending 集合大小，供外部监控。
func (t *WatermarkTracker) PendingCount(topic string, partition int32) int {
	tp := topicPartition{Topic: topic, Partition: partition}
	s := t.getShard(tp)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if p, ok := s.pending[tp]; ok {
		return len(p)
	}
	return 0
}
