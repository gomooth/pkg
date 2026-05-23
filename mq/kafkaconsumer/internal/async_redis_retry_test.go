package internal

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/stretchr/testify/assert"
)

// mockRedisStore 实现 RedisRetryStore 用于单元测试
type mockRedisStore struct {
	items           map[string]*RetryItem
	rescheduleErr   error
	scheduleErr     error
	removeErr       error
	rescheduleCalls atomic.Int32
}

func newMockRedisStore() *mockRedisStore {
	return &mockRedisStore{items: make(map[string]*RetryItem)}
}

func (m *mockRedisStore) itemID(item *RetryItem) string {
	return item.Topic + ":" + string(rune(item.Partition)) + ":" + string(rune(item.Offset))
}

func (m *mockRedisStore) ScheduleRetry(_ context.Context, item *RetryItem) error {
	if m.scheduleErr != nil {
		return m.scheduleErr
	}
	m.items[m.itemID(item)] = item
	return nil
}

func (m *mockRedisStore) FetchPending(_ context.Context, _ time.Time, _ int) ([]*RetryItem, error) {
	return nil, nil
}

func (m *mockRedisStore) RemoveRetry(_ context.Context, item *RetryItem) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	delete(m.items, m.itemID(item))
	return nil
}

func (m *mockRedisStore) AtomicReschedule(_ context.Context, oldItem, newItem *RetryItem) error {
	m.rescheduleCalls.Add(1)
	if m.rescheduleErr != nil {
		return m.rescheduleErr
	}
	delete(m.items, m.itemID(oldItem))
	m.items[m.itemID(newItem)] = newItem
	return nil
}

func (m *mockRedisStore) LoadAllPending(_ context.Context) ([]*RetryItem, error) {
	return nil, nil
}

func (m *mockRedisStore) Close() error { return nil }

func TestAsyncRedisRetry_ProcessRetry_Success(t *testing.T) {
	store := newMockRedisStore()
	var handlerCalls atomic.Int32

	s := &asyncRedisRetry{
		maxRetry:   3,
		backoff:    &retry.FixedDelay{Wait: time.Millisecond},
		handler:    func(_ context.Context, topic string, msg []byte) error { handlerCalls.Add(1); return nil },
		store:      store,
		numWorkers: 1,
		shutdown:   make(chan struct{}),
		logger:     newSlogLogger(nil),
	}

	item := &RetryItem{Topic: "test", Partition: 0, Offset: 1, Attempt: 1, Value: []byte("hello")}
	store.items[store.itemID(item)] = item

	s.processRetry(context.Background(), item)

	assert.Equal(t, int32(1), handlerCalls.Load())
	assert.Empty(t, store.items, "successful retry should remove item from store")
}

func TestAsyncRedisRetry_ProcessRetry_UsesAtomicReschedule(t *testing.T) {
	store := newMockRedisStore()

	s := &asyncRedisRetry{
		maxRetry:   3,
		backoff:    &retry.FixedDelay{Wait: time.Millisecond},
		handler:    func(_ context.Context, topic string, msg []byte) error { return errors.New("fail") },
		store:      store,
		numWorkers: 1,
		shutdown:   make(chan struct{}),
		logger:     newSlogLogger(nil),
	}

	item := &RetryItem{Topic: "test", Partition: 0, Offset: 1, Attempt: 1, Value: []byte("hello")}
	store.items[store.itemID(item)] = item

	s.processRetry(context.Background(), item)

	assert.Equal(t, int32(1), store.rescheduleCalls.Load(), "should call AtomicReschedule on retry")
	assert.Len(t, store.items, 1, "failed retry should reschedule item")
}

func TestAsyncRedisRetry_ProcessRetry_AtomicRescheduleFailure(t *testing.T) {
	store := newMockRedisStore()
	store.rescheduleErr = errors.New("redis down")

	s := &asyncRedisRetry{
		maxRetry:   3,
		backoff:    &retry.FixedDelay{Wait: time.Millisecond},
		handler:    func(_ context.Context, topic string, msg []byte) error { return errors.New("fail") },
		store:      store,
		numWorkers: 1,
		shutdown:   make(chan struct{}),
		logger:     newSlogLogger(nil),
	}

	item := &RetryItem{Topic: "test", Partition: 0, Offset: 1, Attempt: 1, Value: []byte("hello")}
	store.items[store.itemID(item)] = item

	s.processRetry(context.Background(), item)
	assert.Len(t, store.items, 1, "atomic reschedule failure should keep old item")
}

func TestAsyncRedisRetry_ProcessRetry_Exhausted(t *testing.T) {
	store := newMockRedisStore()
	var failedCalled atomic.Int32

	s := &asyncRedisRetry{
		maxRetry: 1,
		backoff:  &retry.FixedDelay{Wait: time.Millisecond},
		handler:  func(_ context.Context, topic string, msg []byte) error { return errors.New("fail") },
		conf: &groupHandlerConf{
			FailedHandler: func(_ context.Context, consumerGroup, topic string, msg []byte, err error) {
				failedCalled.Add(1)
			},
		},
		store:      store,
		numWorkers: 1,
		shutdown:   make(chan struct{}),
		logger:     newSlogLogger(nil),
	}

	item := &RetryItem{Topic: "test", Partition: 0, Offset: 1, Attempt: 2, Value: []byte("hello")}
	store.items[store.itemID(item)] = item

	s.processRetry(context.Background(), item)

	assert.Equal(t, int32(1), failedCalled.Load())
	assert.Empty(t, store.items, "exhausted item should be removed")
}

func TestAsyncRedisRetry_ClearSession_StopsWorkers(t *testing.T) {
	store := newMockRedisStore()
	ctx := context.Background()

	s := &asyncRedisRetry{
		maxRetry:   3,
		backoff:    &retry.FixedDelay{Wait: time.Millisecond},
		handler:    func(_ context.Context, topic string, msg []byte) error { return nil },
		store:      store,
		numWorkers: 1,
		shutdown:   make(chan struct{}),
		logger:     newSlogLogger(nil),
	}

	// 模拟 SetSession 启动 worker
	s.SetSession(&mockSession{ctx: ctx})
	assert.True(t, s.started, "worker should be started after SetSession")

	// 模拟 ClearSession
	s.ClearSession()
	assert.False(t, s.started, "worker should be stopped after ClearSession")
}

func TestAsyncRedisRetry_SetSession_RestartAfterRebalance(t *testing.T) {
	store := newMockRedisStore()
	ctx := context.Background()

	s := &asyncRedisRetry{
		maxRetry:   3,
		backoff:    &retry.FixedDelay{Wait: time.Millisecond},
		handler:    func(_ context.Context, topic string, msg []byte) error { return nil },
		store:      store,
		numWorkers: 1,
		shutdown:   make(chan struct{}),
		logger:     newSlogLogger(nil),
	}

	// 第一次 SetSession
	s.SetSession(&mockSession{ctx: ctx})
	assert.True(t, s.started, "worker should be started after first SetSession")

	// 模拟 Rebalance: ClearSession
	s.ClearSession()
	assert.False(t, s.started, "worker should be stopped after ClearSession")

	// 第二次 SetSession（Rebalance 后）
	s.SetSession(&mockSession{ctx: ctx})
	assert.True(t, s.started, "worker should be restarted after second SetSession")
}
