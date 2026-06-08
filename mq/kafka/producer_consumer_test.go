package kafka

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/gomooth/pkg/mq/kafka/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Producer 测试 ====================

func TestNewProducer_DefaultConfig(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})
	require.NotNil(t, p)

	// Verify IProducer interface
	var _ types.IProducer = p
}

func TestNewProducer_WithOptions(t *testing.T) {
	logger := newTestSlogLogger()
	p := NewProducer([]string{"localhost:9092"},
		WithProducerTimeout(10*time.Second),
		WithProducerLogger(logger),
	)
	require.NotNil(t, p)
}

func TestProducerImpl_StartShutdownCycle(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})

	// Start will fail (no broker), but should not panic
	err := p.Start(context.Background())
	assert.Error(t, err)

	// Shutdown from non-running state should be fine
	err = p.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestProducerImpl_ProduceNotConnected(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})

	// Produce without Start should fail
	err := p.Produce(context.Background(), "test", []byte("hello"))
	assert.Error(t, err)

	err = p.ProduceBatch(context.Background(), "test", [][]byte{[]byte("hello")})
	assert.Error(t, err)
}

func TestProducerImpl_ProduceBatchEmptyMessages(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})
	err := p.ProduceBatch(context.Background(), "test", nil)
	assert.Error(t, err)
}

// ==================== ProducerEngine 扩展测试 ====================

func TestProducerEngine_ProduceWithMock(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

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

	err := eng.Produce(context.Background(), "test-topic", []byte("hello"))
	assert.NoError(t, err)
}

func TestProducerEngine_ProduceBatchWithMock(t *testing.T) {
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

	err := eng.ProduceBatch(context.Background(), "test-topic", [][]byte{[]byte("msg1"), []byte("msg2")})
	assert.NoError(t, err)
}

func TestProducerEngine_ProduceWithOrderKey(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

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

	err := eng.Produce(context.Background(), "test-topic", []byte("hello"), types.WithOrderKey("partition-key"))
	assert.NoError(t, err)
}

func TestProducerEngine_ProduceErrorWithMock(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

	mockProducer.ExpectSendMessageAndFail(errors.New("mock produce error"))

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

	err := eng.Produce(context.Background(), "test-topic", []byte("hello"))
	assert.Error(t, err)
}

func TestProducerEngine_ProduceBatchError(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)

	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      cfg,
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.State.Store(engine.Running)

	// No inner producer
	err := eng.ProduceBatch(context.Background(), "test-topic", [][]byte{[]byte("msg1")})
	assert.Error(t, err)
}

func TestProducerEngine_StartAlreadyRunning(t *testing.T) {
	cfg := internal.BuildProducerConfig(5 * time.Second)
	mockProducer := mocks.NewSyncProducer(t, cfg)
	defer mockProducer.Close()

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

	// Start on already-running engine should return nil
	err := eng.Start(context.Background())
	assert.NoError(t, err)
}

func TestProducerEngine_StartAlreadyClosed(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.State.Store(engine.Closed)

	err := eng.Start(context.Background())
	assert.Error(t, err)
}

func TestProducerEngine_ShutdownWithRunningProducer(t *testing.T) {
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

	_, cancel := context.WithCancel(context.Background())
	eng.CancelFunc = cancel

	err := eng.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int32(engine.Closed), eng.State.Load())
}

func TestProducerEngine_ShutdownFromShuttingDown(t *testing.T) {
	eng := &producerEngine{
		brokers:     []string{"localhost:9092"},
		config:      internal.BuildProducerConfig(5 * time.Second),
		reconnectCh: make(chan struct{}, 1),
	}
	eng.Base = engine.Base{
		Logger:  slog.Default(),
		Metrics: metrics.NewProducerMetrics("kafka"),
	}
	eng.State.Store(engine.ShuttingDown)

	// Already shutting down, should return nil
	err := eng.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestProducerEngine_NewProducerEngineDefaults(t *testing.T) {
	cfg := &producerConfig{timeout: 0}
	eng := newProducerEngine([]string{"broker1:9092"}, cfg)
	require.NotNil(t, eng)
	assert.Equal(t, 5*time.Second, eng.timeout) // default timeout
	assert.NotNil(t, eng.Logger)
	assert.NotNil(t, eng.config)
	assert.NotNil(t, eng.Metrics)
}

func TestProducerEngine_NewProducerEngineCustomTimeout(t *testing.T) {
	logger := newTestSlogLogger()
	cfg := &producerConfig{timeout: 10 * time.Second, logger: logger}
	eng := newProducerEngine([]string{"broker1:9092"}, cfg)
	require.NotNil(t, eng)
	assert.Equal(t, 10*time.Second, eng.timeout)
}

func TestProducerEngine_NewProducerEngineCustomSaramaConfig(t *testing.T) {
	saramaCfg := sarama.NewConfig()
	cfg := &producerConfig{timeout: 5 * time.Second, saramaConfig: saramaCfg}
	eng := newProducerEngine([]string{"broker1:9092"}, cfg)
	require.NotNil(t, eng)
	assert.Equal(t, saramaCfg, eng.config)
}

func TestProducerEngine_ReconnectLoopContextCancel(t *testing.T) {
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

	cancel()

	select {
	case <-done:
		// OK - reconnect loop exited
	case <-time.After(2 * time.Second):
		t.Fatal("reconnectLoop did not exit after context cancel")
	}
}

// ==================== Consumer 测试 ====================

func TestNewConsumer_DefaultConfig(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"})
	require.NotNil(t, c)

	// Verify IConsumeServer interface
	var _ types.IConsumeServer = c
}

func TestNewConsumer_WithOptions(t *testing.T) {
	logger := newTestSlogLogger()
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(logger),
		WithConsumerTimeout(10*time.Second),
		WithMaxRetry(3),
		WithRetryMode(RetryModeAsync),
		WithRetryWorkers(2),
		WithHandlerTimeout(30*time.Second),
	)
	require.NotNil(t, c)
}

func TestConsumerEngine_CountZero(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	assert.Equal(t, uint(0), c.Count())
}

func TestConsumerEngine_StartNoConsumers(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	err := c.Start(context.Background())
	assert.Error(t, err)
	assert.Equal(t, int32(engine.Idle), c.State.Load())
}

func TestConsumerEngine_HealthCheckNotRunning(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	err := c.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestConsumerEngine_HealthCheckRunning(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	c.State.Store(engine.Running)
	err := c.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestConsumerEngine_ShutdownFromIdle(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	err := c.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestConsumerEngine_StartAlreadyRunning(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	c.State.Store(engine.Running)
	err := c.Start(context.Background())
	assert.NoError(t, err) // already running, return nil
}

func TestConsumerEngine_StartAlreadyClosed(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	c.State.Store(engine.Closed)
	err := c.Start(context.Background())
	assert.Error(t, err)
}

func TestConsumerEngine_RegisterRequiresGroup(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	// Register without WithGroup should return error
	err := c.Register("topic1", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WithGroup")
}

func TestConsumerEngine_RegisterAfterStart(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	c.State.Store(engine.Running)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	// Register after start should return error
	err := c.Register("topic", handler, types.WithGroup("group"))
	assert.Error(t, err)
}

func TestConsumerEngine_safeGo(t *testing.T) {
	panicHandled := atomic.Bool{}
	c := NewConsumer([]string{"localhost:9092"},
		WithPanicHandler(func(v any) {
			panicHandled.Store(true)
		}),
	).(*consumerEngine)

	c.SafeGo("test-panic", func() {
		panic("test panic")
	}, c.config.panicHandler)

	time.Sleep(100 * time.Millisecond)
	assert.True(t, panicHandled.Load(), "expected panic handler to be called")
}

func TestConsumerEngine_safeGoNormal(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)

	completed := atomic.Bool{}
	c.SafeGo("test-normal", func() {
		completed.Store(true)
	}, nil)

	time.Sleep(100 * time.Millisecond)
	assert.True(t, completed.Load(), "expected goroutine to complete normally")
}

func TestConsumerEngine_newConsumerEngine(t *testing.T) {
	logger := newTestSlogLogger()
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(logger),
		WithConsumerTimeout(10*time.Second),
		WithMaxRetry(3),
	).(*consumerEngine)

	require.NotNil(t, c)
	assert.Equal(t, []string{"localhost:9092"}, c.brokers)
	assert.NotNil(t, c.config)
	assert.Equal(t, logger, c.config.logger)
	assert.Equal(t, 10*time.Second, c.config.timeout)
	assert.Equal(t, 3, c.config.maxRetry)
}

func TestConsumerEngine_newConsumerEngine_CustomSaramaConfig(t *testing.T) {
	saramaCfg := sarama.NewConfig()
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerSaramaConfig(saramaCfg),
	).(*consumerEngine)

	require.NotNil(t, c)
	assert.Equal(t, saramaCfg, c.config.saramaConfig)
}

func TestConsumerEngine_newConsumerEngine_WithConsumers(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumer("group1", handler, "topic1"),
	).(*consumerEngine)

	require.NotNil(t, c)
	// The registration may fail due to broker unavailability, but no panic
}

func TestConsumerEngine_ShutdownFromRunning(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	c.State.Store(engine.Running)

	err := c.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int32(engine.Closed), c.State.Load())
}

func TestConsumerEngine_HealthCheckStates(t *testing.T) {
	tests := []struct {
		name      string
		state     int32
		wantError bool
	}{
		{"idle", engine.Idle, true},
		{"running", engine.Running, false},
		{"shutting_down", engine.ShuttingDown, true},
		{"closed", engine.Closed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
			c.State.Store(tt.state)
			err := c.HealthCheck(context.Background())
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConsumerEngine_ShutdownWithRegistrations(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithMaxRetry(0),
		WithConsumerSaramaConfig(sarama.NewConfig()),
	).(*consumerEngine)

	// Add a mock registration directly
	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	c.regMu.Lock()
	c.registrations = append(c.registrations, consumerRegistration{
		group:   "test-group",
		topics:  []string{"test-topic"},
		handler: gh,
		cg:      nil, // no real consumer group
	})
	c.regMu.Unlock()

	c.State.Store(engine.Running)
	_, engineCancel := context.WithCancel(context.Background())
	c.CancelFunc = engineCancel

	err := c.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int32(engine.Closed), c.State.Load())
}

func TestConsumerEngine_ShutdownFromShuttingDown(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}).(*consumerEngine)
	c.State.Store(engine.ShuttingDown)

	err := c.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestConsumerEngine_StartWithConsumerGroupError(t *testing.T) {
	// This will fail to create a consumer group (no broker)
	// but tests the error path in createRegistration
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	c := NewConsumer([]string{"invalid:9999"},
		WithConsumerLogger(newTestSlogLogger()),
		WithConsumerSaramaConfig(sarama.NewConfig()),
		WithConsumer("group", handler, "topic"),
	).(*consumerEngine)

	// No registrations should succeed
	assert.Equal(t, uint(0), c.Count())
}

// ==================== GroupHandler 测试 ====================

func TestGroupHandler_Setup(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	session := newMockSession()

	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 3,
	})

	err := gh.Setup(session)
	assert.NoError(t, err)
}

func TestGroupHandler_Cleanup(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	session := newMockSession()

	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 3,
	})

	err := gh.Cleanup(session)
	assert.NoError(t, err)
}

func TestGroupHandler_ConsumeClaim(t *testing.T) {
	var handled atomic.Int32
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		handled.Add(1)
		return nil
	})

	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	session := &mockConsumerGroupSessionWithContext{ctx: ctx}

	msgCh := make(chan *sarama.ConsumerMessage, 1)
	msgCh <- &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	claim := &mockClaimWithChannel{ch: msgCh}

	done := make(chan struct{})
	go func() {
		err := gh.ConsumeClaim(session, claim)
		assert.NoError(t, err)
		close(done)
	}()

	// Wait a bit for the message to be consumed
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), handled.Load())

	// Close channel and cancel context to exit ConsumeClaim
	close(msgCh)
	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("ConsumeClaim did not exit")
	}
}

func TestGroupHandler_ConsumeClaimEmpty(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	session := &mockConsumerGroupSessionWithContext{ctx: ctx}
	// Closed channel - ConsumeClaim should return immediately
	msgCh := make(chan *sarama.ConsumerMessage)
	close(msgCh)
	claim := &mockClaimWithChannel{ch: msgCh}

	err := gh.ConsumeClaim(session, claim)
	assert.NoError(t, err)
	_ = cancel
}

func TestGroupHandler_Shutdown(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	// Should not panic
	gh.Shutdown(context.Background())
}

func TestGroupHandler_AsyncMode(t *testing.T) {
	var handled atomic.Int32
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		handled.Add(1)
		return nil
	})

	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:    handler,
		MaxRetry:   3,
		RetryMode:  RetryModeAsync,
		RetryStore: NewMemoryRetryStore(),
	})

	require.NotNil(t, gh.strategy)

	// Setup and Cleanup should work
	session := newMockSession()
	err := gh.Setup(session)
	assert.NoError(t, err)

	// Shutdown the strategy
	gh.Shutdown(context.Background())
}

func TestGroupHandler_SyncModeWithRetryWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	// maxRetry > 1 should trigger a warning
	_ = newGroupHandler("test-group", &groupHandlerConf{
		Logger:   logger,
		Handler:  handler,
		MaxRetry: 5,
	})

	// Warning should be logged
	assert.Contains(t, buf.String(), "sync retry mode")
}

func TestGroupHandler_DefaultBackoff(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	// No backoff provided - should use default
	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 3,
	})
	require.NotNil(t, gh)
}

// ==================== mockClaim ====================

type mockClaim struct {
	msgs               []*sarama.ConsumerMessage
	closed             bool
	closeAfterMessages bool // close channel after sending all messages
}

func (m *mockClaim) Messages() <-chan *sarama.ConsumerMessage {
	ch := make(chan *sarama.ConsumerMessage, len(m.msgs))
	for _, msg := range m.msgs {
		ch <- msg
	}
	if m.closed || m.closeAfterMessages {
		close(ch)
	}
	return ch
}

func (m *mockClaim) Topic() string {
	if len(m.msgs) > 0 {
		return m.msgs[0].Topic
	}
	return ""
}

func (m *mockClaim) Partition() int32 {
	if len(m.msgs) > 0 {
		return m.msgs[0].Partition
	}
	return 0
}

func (m *mockClaim) InitialOffset() int64      { return 0 }
func (m *mockClaim) HighWaterMarkOffset() int64 { return 0 }

// mockClaimWithChannel uses a pre-created channel for Messages
type mockClaimWithChannel struct {
	ch chan *sarama.ConsumerMessage
}

func (m *mockClaimWithChannel) Messages() <-chan *sarama.ConsumerMessage { return m.ch }
func (m *mockClaimWithChannel) Topic() string                            { return "test" }
func (m *mockClaimWithChannel) Partition() int32                         { return 0 }
func (m *mockClaimWithChannel) InitialOffset() int64                     { return 0 }
func (m *mockClaimWithChannel) HighWaterMarkOffset() int64               { return 0 }

// mockConsumerGroupSessionWithContext supports custom context
type mockConsumerGroupSessionWithContext struct {
	ctx context.Context
}

func (m *mockConsumerGroupSessionWithContext) Claims() map[string][]int32               { return nil }
func (m *mockConsumerGroupSessionWithContext) MemberID() string                         { return "test-member" }
func (m *mockConsumerGroupSessionWithContext) GenerationID() int32                      { return 1 }
func (m *mockConsumerGroupSessionWithContext) MarkOffset(string, int32, int64, string)  {}
func (m *mockConsumerGroupSessionWithContext) Commit()                                  {}
func (m *mockConsumerGroupSessionWithContext) ResetOffset(string, int32, int64, string) {}
func (m *mockConsumerGroupSessionWithContext) MarkMessage(_ *sarama.ConsumerMessage, _ string) {}
func (m *mockConsumerGroupSessionWithContext) Context() context.Context                 { return m.ctx }
func (m *mockConsumerGroupSessionWithContext) Close()                                   {}

// ==================== 辅助函数 ====================

func newTestSlogLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}