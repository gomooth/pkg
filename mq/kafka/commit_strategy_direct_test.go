package kafka

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/stretchr/testify/assert"
)

func TestDirectMarkStrategy_OnSuccess(t *testing.T) {
	store := &nonWatermarkMockStore{}
	strategy := newDirectMarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// OnSuccess should be a no-op for direct mark strategy
	strategy.OnSuccess(context.Background(), session, item)
	// No assertions needed - just verifying it doesn't panic
}

func TestDirectMarkStrategy_OnExhausted(t *testing.T) {
	store := &nonWatermarkMockStore{}
	strategy := newDirectMarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// OnExhausted should be a no-op for direct mark strategy
	strategy.OnExhausted(context.Background(), session, item)
}

func TestDirectMarkStrategy_OnScheduleFailed(t *testing.T) {
	store := &nonWatermarkMockStore{}
	strategy := newDirectMarkStrategy(store, nil)
	session := newMockSession()

	item := &RetryItem{
		Topic:     "test",
		Partition: 0,
		Offset:    5,
	}

	// OnScheduleFailed should be a no-op for direct mark strategy
	strategy.OnScheduleFailed(context.Background(), session, item)
}

func TestDirectMarkStrategy_StartWorkers_ContextCancel(t *testing.T) {
	store := &nonWatermarkMockStore{}
	strategy := newDirectMarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	processFn := func(_ context.Context, _ *RetryItem) {}

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
		t.Fatal("redisPollLoop did not exit after context cancel")
	}
}

func TestDirectMarkStrategy_StartWorkers_ProcessItems(t *testing.T) {
	// Use a store that returns items on first Fetch call, then empty
	store := &fetchReturnItemStore{
		items: []*RetryItem{
			{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
				Attempt: 1, ConsumerGroup: "test-group"},
		},
	}
	strategy := newDirectMarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var processCalls atomic.Int32
	processFn := func(_ context.Context, _ *RetryItem) {
		processCalls.Add(1)
	}

	strategy.StartWorkers(ctx, &wg, processFn)

	// Wait for the poll loop to pick up the item
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, int32(1), processCalls.Load())

	cancel()
	wg.Wait()
}

func TestDirectMarkStrategy_StartWorkers_FetchError(t *testing.T) {
	store := &fetchErrorStore{}
	var buf bytes.Buffer
	logger := logutil.NewSlogLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	strategy := newDirectMarkStrategy(store, logger)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	processFn := func(_ context.Context, _ *RetryItem) {}

	strategy.StartWorkers(ctx, &wg, processFn)

	// Wait a bit for the poll loop to hit the fetch error
	time.Sleep(500 * time.Millisecond)

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("redisPollLoop did not exit")
	}

	// Should have logged the fetch error
	assert.Contains(t, buf.String(), "fetch pending retries failed")
}

func TestDirectMarkStrategy_OnClearSession(t *testing.T) {
	store := &nonWatermarkMockStore{}
	strategy := newDirectMarkStrategy(store, nil)

	// OnClearSession should be a no-op for direct mark strategy
	strategy.OnClearSession()
}

func TestDirectMarkStrategy_OnShutdown(t *testing.T) {
	store := &nonWatermarkMockStore{}
	strategy := newDirectMarkStrategy(store, nil)

	// OnShutdown should be a no-op for direct mark strategy
	strategy.OnShutdown(context.Background())
}

func TestDirectMarkStrategy_InterfaceCompliance(t *testing.T) {
	// 编译时接口检查
	var _ CommitStrategy = (*directMarkStrategy)(nil)
}

func TestDirectMarkStrategy_redisPollLoop_Backoff(t *testing.T) {
	// Test that redisPollLoop backs off when no items are available
	store := &nonWatermarkMockStore{} // always returns empty on Fetch
	strategy := newDirectMarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	processFn := func(_ context.Context, _ *RetryItem) {}

	strategy.StartWorkers(ctx, &wg, processFn)

	// Let it run for a bit to exercise the backoff logic
	time.Sleep(1 * time.Second)

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("redisPollLoop did not exit")
	}
}

func TestDirectMarkStrategy_StartWorkers_MultipleWorkers(t *testing.T) {
	store := &fetchReturnItemStore{
		items: []*RetryItem{
			{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
				Attempt: 1, ConsumerGroup: "test-group"},
		},
	}
	strategy := newDirectMarkStrategy(store, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var processCalls atomic.Int32
	processFn := func(_ context.Context, _ *RetryItem) {
		processCalls.Add(1)
	}

	// Start 2 workers
	strategy.StartWorkers(ctx, &wg, processFn)
	strategy.StartWorkers(ctx, &wg, processFn)

	// Wait for at least one worker to pick up the item
	time.Sleep(500 * time.Millisecond)
	assert.GreaterOrEqual(t, processCalls.Load(), int32(1))

	cancel()
	wg.Wait()
}

func TestDirectMarkStrategy_redisPollLoop_PanicRecovery(t *testing.T) {
	// Test that redisPollLoop recovers from panic
	var panicRecovered atomic.Bool
	store := &panicFetchStore{}
	logger := &panicTestLogger{recovered: &panicRecovered}
	strategy := newDirectMarkStrategy(store, logger)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	processFn := func(_ context.Context, _ *RetryItem) {}

	strategy.StartWorkers(ctx, &wg, processFn)

	// Wait for the panic to be triggered and recovered
	time.Sleep(500 * time.Millisecond)

	cancel()
	wg.Wait()

	assert.True(t, panicRecovered.Load())
}

// panicFetchStore always panics on Fetch
type panicFetchStore struct{}

func (s *panicFetchStore) Schedule(_ context.Context, _ *RetryItem) error { return nil }
func (s *panicFetchStore) Fetch(_ context.Context, _ time.Time, _ int) ([]*RetryItem, error) {
	panic("test panic")
}
func (s *panicFetchStore) Remove(_ context.Context, _ *RetryItem) error                { return nil }
func (s *panicFetchStore) Reschedule(_ context.Context, _, _ *RetryItem) error         { return nil }
func (s *panicFetchStore) LoadAll(_ context.Context) ([]*RetryItem, error)             { return nil, nil }
func (s *panicFetchStore) Close() error                                                { return nil }

// panicTestLogger tracks if Error was called (for panic recovery)
type panicTestLogger struct {
	recovered *atomic.Bool
}

func (l *panicTestLogger) Debug(_ string, _ ...any) {}
func (l *panicTestLogger) Info(_ string, _ ...any)  {}
func (l *panicTestLogger) Warn(_ string, _ ...any)  {}
func (l *panicTestLogger) Error(msg string, _ ...any) {
	if msg == "redisPollLoop panic recovered" {
		l.recovered.Store(true)
	}
}