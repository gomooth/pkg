package kafka

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatermarkStrategy_OnSuccess(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// 先标记 pending，这样 MarkSuccess 才能推进水位线
	store.tracker.MarkPending("test", 0, 5)

	strategy.OnSuccess(context.Background(), session, item)

	wm, ok := store.Watermark("test", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(5), wm)
}

func TestWatermarkStrategy_OnExhausted(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// 先标记 pending
	store.tracker.MarkPending("test", 0, 5)

	strategy.OnExhausted(context.Background(), session, item)

	// RemovePending 后水位线应推进
	wm, ok := store.Watermark("test", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(5), wm)
}

func TestWatermarkStrategy_OnScheduleFailed(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// 先标记 pending 并标记成功，这样水位线才有值
	store.tracker.MarkPending("test", 0, 3)
	store.MarkSuccess("test", 0, 3)

	strategy.OnScheduleFailed(context.Background(), session, item)

	// commitWatermark 应提交水位线
	wm, ok := store.Watermark("test", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(3), wm)
}

func TestWatermarkStrategy_OnScheduleFailed_NoWatermark(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// 没有标记任何 pending，水位线不存在
	strategy.OnScheduleFailed(context.Background(), session, item)
	// 不应 panic，commitWatermark 在 Watermark 返回 false 时不做任何操作
}

func TestWatermarkStrategy_StartWorkers_ContextCancel(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var processCalls atomic.Int32
	processFn := func(_ context.Context, _ *RetryItem) {
		processCalls.Add(1)
	}

	strategy.StartWorkers(ctx, &wg, processFn)

	// Cancel context to stop worker
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK - worker exited
	case <-time.After(3 * time.Second):
		t.Fatal("watermarkWorker did not exit after context cancel")
	}
}

func TestWatermarkStrategy_StartWorkers_ProcessItem(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var processCalls atomic.Int32
	processFn := func(_ context.Context, item *RetryItem) {
		processCalls.Add(1)
	}

	strategy.StartWorkers(ctx, &wg, processFn)

	// Schedule an item that's already due
	now := time.Now()
	item := &RetryItem{
		Topic:         "test",
		Partition:     0,
		Offset:        1,
		Value:         []byte("hello"),
		Attempt:       1,
		NextRetryAt:   now.Add(-time.Second),
		ConsumerGroup: "test-group",
	}
	store.Schedule(context.Background(), item)

	// Wait for the worker to process it
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), processCalls.Load())

	cancel()
	wg.Wait()
}

func TestWatermarkStrategy_OnClearSession(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	// Track some partitions
	strategy.trackPartition("test", 0)
	strategy.trackPartition("test", 1)
	strategy.trackPartition("other", 0)

	// Mark pending so ResetPartition has something to reset
	store.tracker.MarkPending("test", 0, 1)
	store.tracker.MarkPending("test", 1, 2)
	store.tracker.MarkPending("other", 0, 3)

	strategy.OnClearSession()

	// trackedParts should be empty
	strategy.tpMu.Lock()
	assert.Empty(t, strategy.trackedParts)
	strategy.tpMu.Unlock()
}

func TestWatermarkStrategy_OnShutdown(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	// OnShutdown should send a signal to wmStore.Notify()
	notifyCh := store.Notify()

	strategy.OnShutdown(context.Background())

	// The notify channel should receive a signal
	select {
	case <-notifyCh:
		// OK - signal received
	case <-time.After(1 * time.Second):
		t.Fatal("OnShutdown did not send signal to notify channel")
	}
}

func TestWatermarkStrategy_trackPartition(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	strategy.trackPartition("test", 0)
	strategy.trackPartition("test", 1)

	strategy.tpMu.Lock()
	assert.Len(t, strategy.trackedParts, 2)
	assert.True(t, strategy.trackedParts[topicPartition{topic: "test", partition: 0}])
	assert.True(t, strategy.trackedParts[topicPartition{topic: "test", partition: 1}])
	strategy.tpMu.Unlock()
}

func TestCommitWatermark(t *testing.T) {
	store := NewMemoryRetryStore()
	session := newMockSession()

	// Mark pending and success to establish watermark
	store.tracker.MarkPending("test", 0, 5)
	store.MarkSuccess("test", 0, 5)

	commitWatermark(session, "test", 0, store)

	wm, ok := store.Watermark("test", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(5), wm)
}

func TestCommitWatermark_NoWatermark(t *testing.T) {
	store := NewMemoryRetryStore()
	session := newMockSession()

	// No watermark established - should not panic
	commitWatermark(session, "test", 0, store)
}

func TestWatermarkStrategy_InterfaceCompliance(t *testing.T) {
	// 编译时接口检查
	var _ CommitStrategy = (*watermarkStrategy)(nil)
}

func TestWatermarkStrategy_StartWorkers_MultipleWorkers(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var processCalls atomic.Int32
	processFn := func(_ context.Context, _ *RetryItem) {
		processCalls.Add(1)
	}

	// Start 2 workers
	strategy.StartWorkers(ctx, &wg, processFn)
	strategy.StartWorkers(ctx, &wg, processFn)

	// Schedule an item
	now := time.Now()
	item := &RetryItem{
		Topic:         "test",
		Partition:     0,
		Offset:        1,
		Value:         []byte("hello"),
		Attempt:       1,
		NextRetryAt:   now.Add(-time.Second),
		ConsumerGroup: "test-group",
	}
	store.Schedule(context.Background(), item)

	time.Sleep(200 * time.Millisecond)
	// At least one worker should process the item
	assert.GreaterOrEqual(t, processCalls.Load(), int32(1))

	cancel()
	wg.Wait()
}

func TestWatermarkStrategy_OnSuccess_WithSessionMarkOffset(t *testing.T) {
	store := NewMemoryRetryStore()
	strategy := newWatermarkStrategy(store, nil)

	// Use a mock session that tracks MarkOffset calls
	session := &markOffsetTrackingSession{}

	// Mark pending and then OnSuccess
	store.tracker.MarkPending("test", 0, 10)
	strategy.OnSuccess(context.Background(), session, &RetryItem{Topic: "test", Partition: 0, Offset: 10})

	require.Len(t, session.offsets, 1)
	assert.Equal(t, int64(11), session.offsets[0].offset) // wm+1
}

// markOffsetTrackingSession tracks MarkOffset calls for testing
type markOffsetTrackingSession struct {
	offsets []markOffsetCall
}

type markOffsetCall struct {
	topic     string
	partition int32
	offset    int64
}

func (m *markOffsetTrackingSession) Claims() map[string][]int32               { return nil }
func (m *markOffsetTrackingSession) MemberID() string                         { return "test-member" }
func (m *markOffsetTrackingSession) GenerationID() int32                      { return 1 }
func (m *markOffsetTrackingSession) MarkOffset(topic string, partition int32, offset int64, _ string) {
	m.offsets = append(m.offsets, markOffsetCall{topic: topic, partition: partition, offset: offset})
}
func (m *markOffsetTrackingSession) Commit()                                  {}
func (m *markOffsetTrackingSession) ResetOffset(string, int32, int64, string) {}
func (m *markOffsetTrackingSession) MarkMessage(_ *sarama.ConsumerMessage, _ string) {}
func (m *markOffsetTrackingSession) Context() context.Context                 { return context.Background() }
func (m *markOffsetTrackingSession) Close()                                   {}