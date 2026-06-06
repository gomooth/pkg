package kafka

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/kafka/internal"
)

func TestProducerEngine_ProduceSuccess(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

	mockProducer.ExpectSendMessageAndSucceed()

	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
		metrics: metrics.NewProducerMetrics("kafka"),
	}
	engine.mu.Lock()
	engine.inner = mockProducer
	engine.state.Store(producerRunning)
	engine.mu.Unlock()

	err := engine.send(context.Background(), []*sarama.ProducerMessage{
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

	engine := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      cfg,
		reconnectCh: make(chan struct{}, 1),
		metrics:     metrics.NewProducerMetrics("kafka"),
	}
	engine.mu.Lock()
	engine.inner = mockProducer
	engine.state.Store(producerRunning)
	engine.mu.Unlock()

	err := engine.send(context.Background(), []*sarama.ProducerMessage{
		{Topic: "test", Value: sarama.StringEncoder("hello")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProducerEngine_MarkDisconnected(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)

	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	engine.mu.Lock()
	engine.inner = mockProducer
	engine.mu.Unlock()

	engine.markDisconnected()

	engine.mu.RLock()
	inner := engine.inner
	engine.mu.RUnlock()
	if inner != nil {
		t.Error("expected inner to be nil after markDisconnected")
	}
}

func TestProducerEngine_SendNilProducer(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)

	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	// inner is nil by default
	engine.state.Store(producerRunning)

	err := engine.send(context.Background(), []*sarama.ProducerMessage{
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

	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  cfg,
	}
	engine.mu.Lock()
	engine.inner = mockProducer
	engine.state.Store(producerRunning)
	engine.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := engine.send(ctx, []*sarama.ProducerMessage{
		{Topic: "test", Value: sarama.StringEncoder("hello")},
	})
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestProducerEngine_StartStateTransition(t *testing.T) {
	engine := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
		metrics:     metrics.NewProducerMetrics("kafka"),
	}

	// Starting from idle should fail because no broker is available,
	// but the CAS should succeed (transition idle -> attempt running -> revert to idle)
	err := engine.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when no broker available, got nil")
	}

	// After failed start, state should be back to idle
	if engine.state.Load() != producerIdle {
		t.Errorf("expected state %d, got %d", producerIdle, engine.state.Load())
	}
}

func TestProducerEngine_ShutdownFromIdle(t *testing.T) {
	engine := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}

	err := engine.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if engine.state.Load() != producerClosed {
		t.Errorf("expected state %d, got %d", producerClosed, engine.state.Load())
	}
}

func TestProducerEngine_HealthCheck(t *testing.T) {
	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  internal.BuildProducerConfig(5 * time.Second),
	}

	// Not running
	if err := engine.healthCheck(context.Background()); err == nil {
		t.Error("expected error when not running")
	}

	// Running
	engine.state.Store(producerRunning)
	if err := engine.healthCheck(context.Background()); err != nil {
		t.Errorf("unexpected error when running: %v", err)
	}
}

func TestProducerEngine_TriggerReconnect(t *testing.T) {
	engine := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}

	engine.triggerReconnect()

	select {
	case <-engine.reconnectCh:
		// OK
	default:
		t.Error("expected reconnect signal")
	}

	// Second trigger should not block (non-blocking channel send)
	engine.triggerReconnect()
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
	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  internal.BuildProducerConfig(5 * time.Second),
		metrics: metrics.NewProducerMetrics("kafka"),
	}
	engine.state.Store(producerRunning)

	err := engine.ProduceBatch(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty batch, got nil")
	}
}

func TestProducerImpl_ProduceOrderedEmpty(t *testing.T) {
	engine := &producerEngine{
		brokers: []string{"localhost:9092"},
		config:  internal.BuildProducerConfig(5 * time.Second),
		metrics: metrics.NewProducerMetrics("kafka"),
	}
	engine.state.Store(producerRunning)

	err := engine.ProduceOrdered(context.Background(), "test", []byte("key"))
	if err == nil {
		t.Fatal("expected error for empty ordered batch, got nil")
	}
}
