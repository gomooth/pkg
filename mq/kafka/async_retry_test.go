package kafka

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
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

// newTestAsyncRetryEngine 创建测试用异步引擎（无 store）
func newTestAsyncRetryEngine(t *testing.T, maxRetry int, handlerTimeout time.Duration) *asyncRetryEngine {
	handler := FuncHandler(func(ctx context.Context, topic string, message []byte) error {
		return nil
	})
	return newAsyncRetryEngineWithStore("test-group", handler, maxRetry,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		handlerTimeout, 1, nil, nil, nil)
}
