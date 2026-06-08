package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/gomooth/pkg/mq/kafka/internal"
)

func TestProducerEngine_ProduceSuccess(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

	mockProducer.ExpectSendMessageAndSucceed()

	eng := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	eng.Base = engine.Base{
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.mu.Lock()
	eng.inner = mockProducer
	eng.State.Store(engine.Running)
	eng.mu.Unlock()

	err := eng.send(context.Background(), []*sarama.ProducerMessage{
		{Topic: "test", Value: sarama.StringEncoder("hello")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProducerEngine_ProduceError(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

	mockProducer.ExpectSendMessageAndFail(fmt.Errorf("mock error"))

	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      cfg,
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.mu.Lock()
	eng.inner = mockProducer
	eng.State.Store(engine.Running)
	eng.mu.Unlock()

	err := eng.send(context.Background(), []*sarama.ProducerMessage{
		{Topic: "test", Value: sarama.StringEncoder("hello")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProducerEngine_MarkDisconnected(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)

	eng := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	eng.mu.Lock()
	eng.inner = mockProducer
	eng.mu.Unlock()

	eng.markDisconnected()

	eng.mu.RLock()
	inner := eng.inner
	eng.mu.RUnlock()
	if inner != nil {
		t.Error("expected inner to be nil after markDisconnected")
	}
}

func TestProducerEngine_SendNilProducer(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)

	eng := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	// inner is nil by default
	eng.State.Store(engine.Running)

	err := eng.send(context.Background(), []*sarama.ProducerMessage{
		{Topic: "test", Value: sarama.StringEncoder("hello")},
	})
	if err == nil {
		t.Fatal("expected error when producer is nil, got nil")
	}
}

func TestProducerEngine_CancelledContext(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

	eng := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	eng.mu.Lock()
	eng.inner = mockProducer
	eng.State.Store(engine.Running)
	eng.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := eng.send(ctx, []*sarama.ProducerMessage{
		{Topic: "test", Value: sarama.StringEncoder("hello")},
	})
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestProducerEngine_StartStateTransition(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Metrics: metrics.NewProducerMetrics("kafka"),
	}

	// Starting from idle should fail because no broker is available,
	// but the CAS should succeed (transition idle -> attempt running -> revert to idle)
	err := eng.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when no broker available, got nil")
	}

	// After failed start, state should be back to idle
	if eng.State.Load() != engine.Idle {
		t.Errorf("expected state %d, got %d", engine.Idle, eng.State.Load())
	}
}

func TestProducerEngine_ShutdownFromIdle(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}

	err := eng.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if eng.State.Load() != engine.Closed {
		t.Errorf("expected state %d, got %d", engine.Closed, eng.State.Load())
	}
}

func TestProducerEngine_HealthCheck(t *testing.T) {
	eng := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  internal.BuildProducerConfig(5 * time.Second),
	}

	// Not running
	if err := eng.healthCheck(context.Background()); err == nil {
		t.Error("expected error when not running")
	}

	// Running
	eng.State.Store(engine.Running)
	if err := eng.healthCheck(context.Background()); err != nil {
		t.Errorf("unexpected error when running: %v", err)
	}
}

func TestProducerEngine_TriggerReconnect(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}

	eng.triggerReconnect()

	select {
	case <-eng.reconnectCh:
		// OK
	default:
		t.Error("expected reconnect signal")
	}

	// Second trigger should not block (non-blocking channel send)
	eng.triggerReconnect()
}

func TestNewProducer(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})
	if p == nil {
		t.Fatal("expected non-nil producer")
	}

	// Verify it implements IProducer
	var _ IProducer = p
}

func TestNewProducer_WithTimeout(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"}, WithProducerTimeout(10*time.Second))
	if p == nil {
		t.Fatal("expected non-nil producer")
	}
}

func TestProducerImpl_ProduceBatchEmpty(t *testing.T) {
	eng := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  internal.BuildProducerConfig(5 * time.Second),
	}
	eng.Base = engine.Base{
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.State.Store(engine.Running)

	err := eng.ProduceBatch(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error for empty batch, got nil")
	}
}

func TestProducerEngine_ReconnectLoopTriggerAndContextCancel(t *testing.T) {
	// Test that reconnectLoop exits when context is cancelled after a trigger
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	eng.WG.Add(1)
	go func() {
		eng.reconnectLoop(ctx)
		close(done)
	}()

	// Trigger a reconnect - this will try to connect to a non-existent broker
	eng.triggerReconnect()

	// Wait a moment for the reconnect attempt
	time.Sleep(200 * time.Millisecond)

	// Now cancel the context
	cancel()

	select {
	case <-done:
		// OK - reconnectLoop exited
	case <-time.After(3 * time.Second):
		t.Fatal("reconnectLoop did not exit after context cancel")
	}
}

func TestProducerEngine_ReconnectLoopContextCancelBeforeTrigger(t *testing.T) {
	// Test that reconnectLoop exits when context is cancelled without any trigger
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	eng.WG.Add(1)
	go func() {
		eng.reconnectLoop(ctx)
		close(done)
	}()

	// Cancel immediately without triggering reconnect
	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("reconnectLoop did not exit")
	}
}

func TestProducerEngine_MarkDisconnectedNilInner(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}

	// inner is nil, markDisconnected should be safe
	eng.markDisconnected()

	eng.mu.RLock()
	inner := eng.inner
	eng.mu.RUnlock()
	if inner != nil {
		t.Error("expected inner to remain nil")
	}
}

func TestProducerEngine_StartWithMockProducer(t *testing.T) {
	// Test a successful Start followed by Shutdown with a mock producer
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)

	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      cfg,
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.mu.Lock()
	eng.inner = mockProducer
	eng.State.Store(engine.Running)
	eng.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	eng.CancelFunc = cancel

	// Start the reconnect loop
	eng.WG.Add(1)
	go eng.reconnectLoop(ctx)

	// Shutdown should work
	err := eng.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestProducerEngine_ProduceBatchWithOrderKey(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

	mockProducer.ExpectSendMessageAndSucceed()
	mockProducer.ExpectSendMessageAndSucceed()

	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      cfg,
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.mu.Lock()
	eng.inner = mockProducer
	eng.State.Store(engine.Running)
	eng.mu.Unlock()

	err := eng.ProduceBatch(context.Background(), "test-topic", [][]byte{[]byte("msg1"), []byte("msg2")},
		types.WithOrderKey("partition-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProducerEngine_TriggerReconnectNonBlocking(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}

	// First trigger should succeed
	eng.triggerReconnect()

	// Second trigger should not block (channel already has one signal)
	done := make(chan struct{})
	go func() {
		eng.triggerReconnect()
		close(done)
	}()

	select {
	case <-done:
		// OK - did not block
	case <-time.After(2 * time.Second):
		t.Fatal("triggerReconnect blocked")
	}
}
