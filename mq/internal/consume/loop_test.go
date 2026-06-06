package consume

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace/noop"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// testFetcher implements Fetcher for testing
type testFetcher struct {
	mu      sync.Mutex
	results []FetchResult
	index   int
	// dynamic mode: if Dynamic is set, Fetch returns from Dynamic channel
	dynamic <-chan FetchResult
}

func (f *testFetcher) Fetch(_ context.Context) FetchResult {
	if f.dynamic != nil {
		select {
		case r, ok := <-f.dynamic:
			if !ok {
				return FetchResult{Empty: true}
			}
			return r
		default:
			return FetchResult{Empty: true}
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.index >= len(f.results) {
		return FetchResult{Empty: true}
	}
	r := f.results[f.index]
	f.index++
	return r
}

// testStrategy implements RetryStrategy for testing
type testStrategy struct {
	mu       sync.Mutex
	messages []string
	err      error // if set, OnMessage returns this error
}

func (s *testStrategy) OnMessage(_ context.Context, _ string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, string(data))
	return s.err
}

func (s *testStrategy) getMessages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.messages))
	copy(cp, s.messages)
	return cp
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestConsumeLoop_EmptyQueue(t *testing.T) {
	// Fetcher returns empty results, context is canceled after a short time
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	fetcher := &testFetcher{
		results: []FetchResult{}, // always empty
	}
	strategy := &testStrategy{}

	cfg := LoopConfig{
		MQSystem:   "redis",
		QueueName:  "test-queue",
		EmptySleep: 10 * time.Millisecond,
		Tracer:     noop.NewTracerProvider().Tracer("test"),
	}

	ConsumeLoop(ctx, cfg, fetcher, strategy)

	// Should exit cleanly when context is canceled
	msgs := strategy.getMessages()
	assert.Empty(t, msgs, "no messages should be processed on empty queue")
}

func TestConsumeLoop_ProcessMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a channel-based fetcher so we can control exactly when messages appear
	ch := make(chan FetchResult, 10)
	fetcher := &testFetcher{dynamic: ch}
	strategy := &testStrategy{}

	cfg := LoopConfig{
		MQSystem:   "redis",
		QueueName:  "test-queue",
		EmptySleep: 10 * time.Millisecond,
		Tracer:     noop.NewTracerProvider().Tracer("test"),
	}

	done := make(chan struct{})
	go func() {
		ConsumeLoop(ctx, cfg, fetcher, strategy)
		close(done)
	}()

	// Send a few messages
	ch <- FetchResult{Data: `{"msg":"hello"}`}
	ch <- FetchResult{Data: `{"msg":"world"}`}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop the loop
	cancel()
	<-done

	msgs := strategy.getMessages()
	assert.Equal(t, 2, len(msgs), "should process both messages")
	assert.Contains(t, msgs[0], "hello")
	assert.Contains(t, msgs[1], "world")
}

func TestConsumeLoop_ErrorBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan FetchResult, 20)
	fetcher := &testFetcher{dynamic: ch}
	strategy := &testStrategy{}

	cfg := LoopConfig{
		MQSystem:      "redis",
		QueueName:     "test-queue",
		EmptySleep:    10 * time.Millisecond,
		MaxErrors:     5,
		PauseDuration: 50 * time.Millisecond,
		Backoff:       &fixedBackoff{delay: 5 * time.Millisecond},
		Tracer:        noop.NewTracerProvider().Tracer("test"),
	}

	done := make(chan struct{})
	go func() {
		ConsumeLoop(ctx, cfg, fetcher, strategy)
		close(done)
	}()

	// Send errors, then a successful message
	ch <- FetchResult{Err: errors.New("connection refused")}
	ch <- FetchResult{Err: errors.New("connection refused")}
	ch <- FetchResult{Data: `{"msg":"recovered"}`}

	// Give time for processing
	time.Sleep(200 * time.Millisecond)

	cancel()
	<-done

	msgs := strategy.getMessages()
	assert.Equal(t, 1, len(msgs), "should process the recovered message")
	assert.Contains(t, msgs[0], "recovered")
}

func TestConsumeLoop_ContextCanceledImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	fetcher := &testFetcher{
		results: []FetchResult{{Data: `{"msg":"should not process"}`}},
	}
	strategy := &testStrategy{}

	cfg := LoopConfig{
		MQSystem:   "redis",
		QueueName:  "test-queue",
		EmptySleep: 10 * time.Millisecond,
		Tracer:     noop.NewTracerProvider().Tracer("test"),
	}

	// Should return quickly
	ConsumeLoop(ctx, cfg, fetcher, strategy)

	msgs := strategy.getMessages()
	assert.Empty(t, msgs, "no messages should be processed when context is already canceled")
}

func TestConsumeLoop_StrategyErrorDoesNotStopLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan FetchResult, 20)
	fetcher := &testFetcher{dynamic: ch}
	strategy := &testStrategy{err: errors.New("processing failed")}

	cfg := LoopConfig{
		MQSystem:   "redis",
		QueueName:  "test-queue",
		EmptySleep: 10 * time.Millisecond,
		Tracer:     noop.NewTracerProvider().Tracer("test"),
	}

	var callCount atomic.Int32

	done := make(chan struct{})
	go func() {
		ConsumeLoop(ctx, cfg, fetcher, &countingStrategy{inner: strategy, count: &callCount})
		close(done)
	}()

	// Send two messages — strategy returns errors but loop should continue
	ch <- FetchResult{Data: `{"msg":"first"}`}
	ch <- FetchResult{Data: `{"msg":"second"}`}

	time.Sleep(100 * time.Millisecond)

	cancel()
	<-done

	assert.Equal(t, int32(2), callCount.Load(), "strategy should be called twice despite errors")
}

// countingStrategy wraps a RetryStrategy and counts calls
type countingStrategy struct {
	inner RetryStrategy
	count *atomic.Int32
}

func (s *countingStrategy) OnMessage(ctx context.Context, queue string, data []byte) error {
	s.count.Add(1)
	return s.inner.OnMessage(ctx, queue, data)
}

// fixedBackoff is a simple BackoffStrategy for testing that always returns the same delay
type fixedBackoff struct {
	delay time.Duration
}

func (f *fixedBackoff) Delay(_ uint) time.Duration { return f.delay }
