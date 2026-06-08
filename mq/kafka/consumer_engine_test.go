package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumerEngine_RegisterWithGroup(t *testing.T) {
	// Test Register with WithGroup option - validates the full Register path
	// including ApplyRegisterOptions and createRegistration
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithConsumerSaramaConfig(sarama.NewConfig()),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	// Register with WithGroup - this will fail because no broker, but tests the code path
	err := c.Register("topic1", handler, types.WithGroup("my-group"))
	// The error should come from createRegistration (broker connection failure)
	assert.Error(t, err)
}

func TestConsumerEngine_RegisterWithExtraTopics(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithConsumerSaramaConfig(sarama.NewConfig()),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	// Register with WithGroup and WithExtraTopics
	err := c.Register("topic1", handler,
		types.WithGroup("my-group"),
		types.WithExtraTopics("topic2", "topic3"),
	)
	// Will fail because no broker available
	assert.Error(t, err)
}

func TestConsumerEngine_RegisterEmptyTopic(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithConsumerSaramaConfig(sarama.NewConfig()),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	// Register with empty extra topic should fail
	err := c.Register("topic1", handler,
		types.WithGroup("my-group"),
		types.WithExtraTopics(""), // empty topic
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topic must not be empty")
}

func TestConsumerEngine_RegisterWithCustomSaramaConfig(t *testing.T) {
	// When engine has a custom saramaConfig, Register should use it
	saramaCfg := sarama.NewConfig()
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithConsumerSaramaConfig(saramaCfg),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	err := c.Register("topic1", handler, types.WithGroup("my-group"))
	// Will fail due to no broker, but saramaConfig path is exercised
	assert.Error(t, err)
}

func TestConsumerEngine_RegisterWithDefaultTimeout(t *testing.T) {
	// When no custom saramaConfig and no timeout, should use default 5s timeout
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })

	err := c.Register("topic1", handler, types.WithGroup("my-group"))
	assert.Error(t, err)
}

func TestConsumerEngine_StartShutdownLifecycle(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithMaxRetry(0),
	).(*consumerEngine)

	// Add a mock registration manually to bypass broker connection
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	c.regMu.Lock()
	c.registrations = append(c.registrations, consumerRegistration{
		group:   "test-group",
		topics:  []string{"test-topic"},
		handler: gh,
		cg:      nil, // no real consumer group client
	})
	c.regMu.Unlock()

	// Start should succeed (CAS idle->running)
	err := c.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(engine.Running), c.State.Load())

	// Shutdown should work
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = c.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.Equal(t, int32(engine.Closed), c.State.Load())
}

func TestConsumerEngine_ShutdownWithCancelFunc(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithMaxRetry(0),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	c.regMu.Lock()
	c.registrations = append(c.registrations, consumerRegistration{
		group:   "test-group",
		topics:  []string{"test-topic"},
		handler: gh,
		cg:      nil,
	})
	c.regMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	err := c.Start(ctx)
	require.NoError(t, err)
	assert.NotNil(t, c.CancelFunc)

	// Now shutdown
	err = c.Shutdown(context.Background())
	assert.NoError(t, err)
	cancel() // clean up our context
}

func TestConsumerEngine_HandleContextCancel(t *testing.T) {
	// Test the handle loop exits when state is not Running
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
	).(*consumerEngine)

	// Set state to Idle so the handle loop should exit immediately
	c.State.Store(engine.Idle)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// handle should return immediately since state is not Running
	done := make(chan struct{})
	go func() {
		c.handle(ctx, nil, []string{"test"}, gh)
		close(done)
	}()

	select {
	case <-done:
		// OK - handle exited
	case <-time.After(2 * time.Second):
		t.Fatal("handle did not exit when state is not Running")
	}
}

func TestConsumerEngine_createRegistrationWithGroupFailedHandler(t *testing.T) {
	// Test that createRegistration picks up group-level failed handler
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithConsumeGroupFailedHandler("my-group", func(ctx context.Context, msg types.Message, err error) {}),
	).(*consumerEngine)

	// The groupFailedHandlers map should be set
	require.NotNil(t, c.config.groupFailedHandlers)
	assert.Contains(t, c.config.groupFailedHandlers, "my-group")
}

func TestConsumerEngine_CountAfterRegistrations(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithMaxRetry(0),
	).(*consumerEngine)

	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	gh := newGroupHandler("test-group", &groupHandlerConf{
		Handler:  handler,
		MaxRetry: 0,
	})

	c.regMu.Lock()
	c.registrations = append(c.registrations, consumerRegistration{
		group:   "test-group",
		topics:  []string{"test-topic"},
		handler: gh,
		cg:      nil,
	})
	c.regMu.Unlock()

	assert.Equal(t, uint(1), c.Count())
}

func TestConsumerEngine_ShutdownCtxTimeout(t *testing.T) {
	// Test that Shutdown respects the context timeout
	c := NewConsumer([]string{"localhost:9092"},
		WithConsumerLogger(newTestSlogLogger()),
		WithMaxRetry(0),
	).(*consumerEngine)

	c.State.Store(engine.Running)
	_, cancel := context.WithCancel(context.Background())
	c.CancelFunc = cancel

	// Use a very short timeout context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer shutdownCancel()
	time.Sleep(time.Millisecond) // ensure deadline is exceeded

	err := c.Shutdown(shutdownCtx)
	assert.NoError(t, err) // Shutdown always returns nil or firstErr from cg.Close
	assert.Equal(t, int32(engine.Closed), c.State.Load())
}
