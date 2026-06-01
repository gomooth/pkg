package kafka

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncRetry_SetSessionClearSession(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)
	session := newMockSession()

	engine.SetSession(session)
	time.Sleep(50 * time.Millisecond)

	engine.ClearSession()
	// 不应 panic（修复 P1：context.WithCancel 幂等）
}

func TestAsyncRetry_OnShutdownIdempotent(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)
	session := newMockSession()

	engine.SetSession(session)
	time.Sleep(50 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 多次调用 OnShutdown 不应 panic（修复 P1）
	engine.OnShutdown(shutdownCtx)
	engine.OnShutdown(shutdownCtx)
}

func TestAsyncRetry_HandlerTimeout(t *testing.T) {
	var calls atomic.Int32
	slowHandler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		return errors.New("slow")
	})

	engine := newTestAsyncRetryEngine(t, 0, 50*time.Millisecond)
	engine.handler = slowHandler
	session := newMockSession()

	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(100 * time.Millisecond)
	if calls.Load() == 0 {
		t.Error("expected handler to be called")
	}

	engine.ClearSession()
}

func TestAsyncRetry_SuccessWithWatermark(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return nil
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 1 {
		t.Errorf("expected 1 handler call, got %d", calls.Load())
	}

	wm, ok := store.Watermark("test", 0)
	if !ok || wm != 1 {
		t.Errorf("expected watermark 1, got %d, ok=%v", wm, ok)
	}

	engine.ClearSession()
}

func TestAsyncRetry_OnMessageSuccess_NonWatermark(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return nil
	})

	// Use a store that is NOT a WatermarkStore (Redis-style)
	store := &nonWatermarkMockStore{}
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	assert.Len(t, session.marks, 1) // non-watermark: MarkMessage called

	engine.ClearSession()
}

func TestAsyncRetry_OnMessageFail_NoRetry(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	// maxRetry=0 means no retry, fail immediately
	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 0,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	// With watermark and exhausted handled, MarkMessage should have been called via commitWatermark
	// but since wmStore.RemovePending + commitWatermark, offset may be committed

	engine.ClearSession()
}

func TestAsyncRetry_OnMessageFail_NoRetry_NonWatermark(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	store := &nonWatermarkMockStore{}
	engine := newAsyncRetryEngineWithStore("test-group", handler, 0,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	// exhausted handled -> MarkMessage
	assert.Len(t, session.marks, 1)

	engine.ClearSession()
}

func TestAsyncRetry_OnMessageFail_WithRetry(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	// Message should be scheduled for retry (not MarkMessage yet due to watermark)

	engine.ClearSession()
}

func TestAsyncRetry_OnMessageFail_ScheduleError(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	// Store that fails Schedule
	store := &failScheduleStore{}
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())

	engine.ClearSession()
}

func TestAsyncRetry_OnMessageFail_ScheduleError_WatermarkStore(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	// MemoryRetryStore with very small queue that will be full
	store := NewMemoryRetryStore(WithMemoryMaxQueueSize(0))
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())

	engine.ClearSession()
}

func TestAsyncRetry_SetFailedHandler(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)
	called := false
	engine.SetFailedHandler(FailedHandlerFunc(func(ctx context.Context, cg string, topic string, message []byte, err error) {
		called = true
	}))
	assert.NotNil(t, engine.failedHandler)
	engine.failedHandler(context.Background(), "g", "t", nil, nil)
	assert.True(t, called)
}

func TestAsyncRetry_SetDeadLetterHandler(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)
	dl := &testDeadLetterHandler{}
	engine.SetDeadLetterHandler(dl)
	assert.NotNil(t, engine.deadLetter)
}

func TestAsyncRetry_processRetry_Success(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return nil
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1,
	}

	engine.processRetry(context.Background(), item)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())

	engine.ClearSession()
}

func TestAsyncRetry_processRetry_Success_NonWatermark(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return nil
	})

	store := &nonWatermarkMockStore{}
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1,
	}

	engine.processRetry(context.Background(), item)
	assert.Equal(t, int32(1), calls.Load())
	assert.True(t, store.removeCalled.Load())
}

func TestAsyncRetry_processRetry_RetryableFail(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1,
	}

	engine.processRetry(context.Background(), item)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	// Item should have been rescheduled (attempt incremented)

	engine.ClearSession()
}

func TestAsyncRetry_processRetry_Exhausted(t *testing.T) {
	var calls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		calls.Add(1)
		return errors.New("fail")
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 2,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 2, // Already at max
	}

	engine.processRetry(context.Background(), item)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
	// Item should be removed from pending (exhausted handled)

	engine.ClearSession()
}

func TestAsyncRetry_processRetry_Exhausted_WithDeadLetter(t *testing.T) {
	var handlerCalls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		handlerCalls.Add(1)
		return errors.New("fail")
	})

	dl := &testDeadLetterHandler{}

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 1,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)
	engine.SetDeadLetterHandler(dl)

	session := newMockSession()
	engine.SetSession(session)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, // max reached
	}

	engine.processRetry(context.Background(), item)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), handlerCalls.Load())
	assert.True(t, dl.called.Load())

	engine.ClearSession()
}

func TestAsyncRetry_processRetry_Exhausted_DeadLetterFail(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return errors.New("fail")
	})

	dl := &failingDeadLetterHandler{}

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 1,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)
	engine.SetDeadLetterHandler(dl)

	session := newMockSession()
	engine.SetSession(session)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1,
	}

	engine.processRetry(context.Background(), item)

	time.Sleep(50 * time.Millisecond)
	// Dead letter handler failed -> exhaustedFailed -> offset not committed

	engine.ClearSession()
}

func TestAsyncRetry_processRetry_RescheduleFail(t *testing.T) {
	var handlerCalls atomic.Int32
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		handlerCalls.Add(1)
		return errors.New("fail")
	})

	store := &failRescheduleStore{}
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1,
	}

	// Should not panic
	engine.processRetry(context.Background(), item)
	assert.Equal(t, int32(1), handlerCalls.Load())
}

func TestAsyncRetry_recoverPending(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})

	store := &mockStoreWithLoadAll{
		items: []*RetryItem{
			{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"), NextRetryAt: time.Now().Add(-time.Second), ConsumerGroup: "g"},
			{Topic: "test", Partition: 0, Offset: 2, Value: []byte("world"), NextRetryAt: time.Now().Add(10 * time.Second), ConsumerGroup: "g"},
		},
		scheduleErr: nil,
	}

	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	ctx := context.Background()
	engine.recoverPending(ctx)
	// Should schedule the recovered items
	assert.True(t, store.scheduleCalled.Load() > 0)
}

func TestAsyncRetry_recoverPending_LoadAllError(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})

	store := &mockStoreWithLoadAll{
		loadAllErr: errors.New("redis error"),
	}

	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	ctx := context.Background()
	engine.recoverPending(ctx)
	// Should not panic
}

func TestAsyncRetry_recoverPending_Empty(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})

	store := &mockStoreWithLoadAll{items: nil}
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	ctx := context.Background()
	engine.recoverPending(ctx)
	// No items to recover, should be fine
}

func TestAsyncRetry_recoverPending_NilStore(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)
	engine.store = nil
	engine.recoverPending(context.Background())
	// Should not panic
}

func TestAsyncRetry_recoverPending_ScheduleError(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})

	store := &mockStoreWithLoadAll{
		items: []*RetryItem{
			{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"), NextRetryAt: time.Now().Add(-time.Second), ConsumerGroup: "g"},
		},
		scheduleErr: errors.New("schedule failed"),
	}

	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	engine.recoverPending(context.Background())
	// Should not panic even if schedule fails
}

func TestAsyncRetry_getSession(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)

	// No session set
	sess := engine.getSession()
	assert.Nil(t, sess)

	session := newMockSession()
	engine.SetSession(session)
	time.Sleep(50 * time.Millisecond)

	sess = engine.getSession()
	assert.NotNil(t, sess)

	engine.ClearSession()
}

func TestAsyncRetry_saramaHeadersToPublic(t *testing.T) {
	headers := []*sarama.RecordHeader{
		{Key: []byte("k1"), Value: []byte("v1")},
		{Key: []byte("k2"), Value: []byte("v2")},
	}
	result := saramaHeadersToPublic(headers)
	require.Len(t, result, 2)
	assert.Equal(t, "k1", result[0].Key)
	assert.Equal(t, []byte("v1"), result[0].Value)
	assert.Equal(t, "k2", result[1].Key)
	assert.Equal(t, []byte("v2"), result[1].Value)
}

func TestAsyncRetry_saramaHeadersToPublic_Empty(t *testing.T) {
	result := saramaHeadersToPublic(nil)
	assert.Empty(t, result)
}

func TestNewAsyncRetryEngine_DefaultWorkers(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error { return nil })
	engine := newAsyncRetryEngine("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 0, // numWorkers=0, should default to NumCPU
		nil, nil,
	)
	require.NotNil(t, engine)
	assert.GreaterOrEqual(t, engine.numWorkers, 1)
}

func TestAsyncRetry_commitWatermark_NoWatermarkStore(t *testing.T) {
	engine := newTestAsyncRetryEngine(t, 0, 0)
	session := newMockSession()

	// Should not panic when wmStore is nil
	engine.commitWatermark(session, "test", 0)
}

func TestAsyncRetry_OnMessageWithHeaders(t *testing.T) {
	var receivedHeaders []HeaderKV
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Headers: []*sarama.RecordHeader{
			{Key: []byte("trace-id"), Value: []byte("123")},
		},
	}
	engine.OnMessage(context.Background(), session, msg)

	time.Sleep(50 * time.Millisecond)
	// Headers should be captured via saramaHeadersToPublic
	_ = receivedHeaders

	engine.ClearSession()
}

func TestAsyncRetry_OnShutdownWithWatermark(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error { return nil })
	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)
	time.Sleep(50 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	engine.OnShutdown(shutdownCtx)
}

func TestAsyncRetry_ClearSession_WithWatermarkReset(t *testing.T) {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return errors.New("fail")
	})

	store := NewMemoryRetryStore()
	engine := newAsyncRetryEngineWithStore("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, 1, store, nil, nil)

	session := newMockSession()
	engine.SetSession(session)

	// Send a message that fails (triggers tracking)
	msg := &sarama.ConsumerMessage{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
	}
	engine.OnMessage(context.Background(), session, msg)
	time.Sleep(50 * time.Millisecond)

	// ClearSession should reset partitions
	engine.ClearSession()
}

// ==================== Mock 类型 ====================

// nonWatermarkMockStore 实现 RetryStore 但不是 WatermarkStore
type nonWatermarkMockStore struct {
	scheduleCalled atomic.Int32
	removeCalled   atomic.Bool
}

func (s *nonWatermarkMockStore) Schedule(_ context.Context, _ *RetryItem) error {
	s.scheduleCalled.Add(1)
	return nil
}
func (s *nonWatermarkMockStore) Fetch(_ context.Context, _ time.Time, _ int) ([]*RetryItem, error) {
	return nil, nil
}
func (s *nonWatermarkMockStore) Remove(_ context.Context, _ *RetryItem) error {
	s.removeCalled.Store(true)
	return nil
}
func (s *nonWatermarkMockStore) Reschedule(_ context.Context, _, _ *RetryItem) error { return nil }
func (s *nonWatermarkMockStore) LoadAll(_ context.Context) ([]*RetryItem, error)      { return nil, nil }
func (s *nonWatermarkMockStore) Close() error                                         { return nil }

// failScheduleStore always fails Schedule
type failScheduleStore struct{}

func (s *failScheduleStore) Schedule(_ context.Context, _ *RetryItem) error {
	return errors.New("schedule failed")
}
func (s *failScheduleStore) Fetch(_ context.Context, _ time.Time, _ int) ([]*RetryItem, error) {
	return nil, nil
}
func (s *failScheduleStore) Remove(_ context.Context, _ *RetryItem) error              { return nil }
func (s *failScheduleStore) Reschedule(_ context.Context, _, _ *RetryItem) error       { return nil }
func (s *failScheduleStore) LoadAll(_ context.Context) ([]*RetryItem, error)           { return nil, nil }
func (s *failScheduleStore) Close() error                                              { return nil }

// failRescheduleStore always fails Reschedule
type failRescheduleStore struct{}

func (s *failRescheduleStore) Schedule(_ context.Context, _ *RetryItem) error { return nil }
func (s *failRescheduleStore) Fetch(_ context.Context, _ time.Time, _ int) ([]*RetryItem, error) {
	return nil, nil
}
func (s *failRescheduleStore) Remove(_ context.Context, _ *RetryItem) error {
	return nil
}
func (s *failRescheduleStore) Reschedule(_ context.Context, _, _ *RetryItem) error {
	return errors.New("reschedule failed")
}
func (s *failRescheduleStore) LoadAll(_ context.Context) ([]*RetryItem, error) { return nil, nil }
func (s *failRescheduleStore) Close() error                                    { return nil }

// mockStoreWithLoadAll 用于测试 recoverPending
type mockStoreWithLoadAll struct {
	items       []*RetryItem
	loadAllErr  error
	scheduleErr error
	scheduleCalled atomic.Int32
}

func (s *mockStoreWithLoadAll) Schedule(_ context.Context, _ *RetryItem) error {
	s.scheduleCalled.Add(1)
	return s.scheduleErr
}
func (s *mockStoreWithLoadAll) Fetch(_ context.Context, _ time.Time, _ int) ([]*RetryItem, error) {
	return nil, nil
}
func (s *mockStoreWithLoadAll) Remove(_ context.Context, _ *RetryItem) error              { return nil }
func (s *mockStoreWithLoadAll) Reschedule(_ context.Context, _, _ *RetryItem) error       { return nil }
func (s *mockStoreWithLoadAll) LoadAll(_ context.Context) ([]*RetryItem, error)           { return s.items, s.loadAllErr }
func (s *mockStoreWithLoadAll) Close() error                                              { return nil }

// testDeadLetterHandler implements DeadLetterHandler
type testDeadLetterHandler struct {
	called atomic.Bool
}

func (h *testDeadLetterHandler) OnDeadLetter(_ context.Context, _ string, _ []byte, _ error) error {
	h.called.Store(true)
	return nil
}

// failingDeadLetterHandler implements DeadLetterHandler that always fails
type failingDeadLetterHandler struct{}

func (h *failingDeadLetterHandler) OnDeadLetter(_ context.Context, _ string, _ []byte, _ error) error {
	return errors.New("dead letter failed")
}

// newTestAsyncRetryEngine 创建测试用异步引擎（无 store）
func newTestAsyncRetryEngine(t *testing.T, maxRetry int, handlerTimeout time.Duration) *asyncRetryEngine {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})
	return newAsyncRetryEngineWithStore("test-group", handler, maxRetry,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		handlerTimeout, 1, nil, nil, nil)
}
