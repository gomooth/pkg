package httpsqs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/httpsqs/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Option Tests ====================

func TestWithHandlerTimeout(t *testing.T) {
	cfg := &consumerConfig{}
	WithHandlerTimeout(5 * time.Second)(cfg)
	assert.Equal(t, 5*time.Second, cfg.handlerTimeout)
}

func TestWithPanicHandler(t *testing.T) {
	var panicVal any
	cfg := &consumerConfig{}
	WithPanicHandler(func(v any) {
		panicVal = v
	})(cfg)
	assert.NotNil(t, cfg.panicHandler)
	cfg.panicHandler("test-panic")
	assert.Equal(t, "test-panic", panicVal)
}

func TestWithConsumers(t *testing.T) {
	cfg := &consumerConfig{}
	regs := []ConsumerRegistration{
		{Queue: "q1", Handler: FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil })},
		{Queue: "q2", Handler: FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil })},
	}
	WithConsumers(regs...)(cfg)
	assert.Len(t, cfg.consumers, 2)
}

func TestWithConsumerLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cfg := &consumerConfig{}
	WithConsumerLogger(logger)(cfg)
	assert.Equal(t, logger, cfg.logger)
}

func TestWithQueueHTTPSQSClient(t *testing.T) {
	client := &mockGetClient{}
	cfg := &queueConfig{}
	WithQueueHTTPSQSClient(client)(cfg)
	assert.Equal(t, client, cfg.client)
}

func TestWithQueueMaxRetry(t *testing.T) {
	cfg := &queueConfig{}
	WithQueueMaxRetry(5)(cfg)
	assert.NotNil(t, cfg.maxRetry)
	assert.Equal(t, 5, *cfg.maxRetry)
}

func TestWithQueueBackoff(t *testing.T) {
	bo := &retry.FixedDelay{Wait: time.Second}
	cfg := &queueConfig{}
	WithQueueBackoff(bo)(cfg)
	assert.Equal(t, bo, cfg.backoff)
}

func TestWithQueueRetryMode(t *testing.T) {
	cfg := &queueConfig{}
	WithQueueRetryMode(RetryModeRequeue)(cfg)
	assert.NotNil(t, cfg.retryMode)
	assert.Equal(t, RetryModeRequeue, *cfg.retryMode)
}

// ==================== Register Tests ====================

func TestConsumer_RegisterAfterStart(t *testing.T) {
	client := &mockGetClient{
		results: []mockGetResult{},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithConsumerLogger(logger),
	)

	err := consumer.Start(context.Background())
	require.NoError(t, err)

	// Register after start should log error and skip
	consumer.Register("q2", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil }))
	assert.Contains(t, buf.String(), "cannot register after consumer started")
	assert.Equal(t, uint(1), consumer.Count())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

func TestConsumer_RegisterBeforeStart(t *testing.T) {
	client := &mockGetClient{}

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())

	// Register before start should succeed
	consumer.Register("q2", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil }))
	assert.Equal(t, uint(2), consumer.Count())
}

func TestConsumer_RegisterEmptyQueueName(t *testing.T) {
	client := &mockGetClient{}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumerLogger(logger),
	)

	// Register with empty queue name should log error
	consumer.Register("", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error { return nil }))
	assert.Contains(t, buf.String(), "queue name must not be empty")
	assert.Equal(t, uint(0), consumer.Count())
}

func TestConsumer_RegisterNilHandler(t *testing.T) {
	client := &mockGetClient{}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumerLogger(logger),
	)

	// Register with nil handler should log error
	consumer.Register("q1", nil)
	assert.Contains(t, buf.String(), "handler must not be nil")
	assert.Equal(t, uint(0), consumer.Count())
}

// ==================== SetDeadLetterHandler Tests ====================

func TestSyncRetryStrategy_SetDeadLetterHandler(t *testing.T) {
	var dlCalled atomic.Int32

	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("always fail")
		}),
		0,
		&retry.FixedDelay{Wait: time.Millisecond},
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	dl := httpsqsDeadLetterFunc(func(ctx context.Context, queue string, data string, pos int64, lastErr error) error {
		dlCalled.Add(1)
		return nil
	})
	strategy.SetDeadLetterHandler(dl)

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), dlCalled.Load())
}

func TestRequeueRetryStrategy_SetDeadLetterHandler(t *testing.T) {
	client := &mockHTTPSQSClient{}

	var dlCalled atomic.Int32

	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("always fail")
		}),
		0, // maxRetry=0, immediate exhaustion
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"test-queue",
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	dl := httpsqsDeadLetterFunc(func(ctx context.Context, queue string, data string, pos int64, lastErr error) error {
		dlCalled.Add(1)
		return nil
	})
	strategy.SetDeadLetterHandler(dl)

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), dlCalled.Load())
}

// ==================== handleExhausted with dead letter error ====================

func TestHandleExhausted_DeadLetterError(t *testing.T) {
	var buf bytes.Buffer
	logger := internal.NewSlogLogger(slog.New(slog.NewTextHandler(&buf, nil)))

	result := handleExhausted(
		context.Background(),
		"test",
		"hello",
		1,
		errors.New("fail"),
		httpsqsDeadLetterFunc(func(ctx context.Context, queue string, data string, pos int64, lastErr error) error {
			return errors.New("dead letter handler failed")
		}),
		nil,
		logger,
		nil,
	)

	assert.Equal(t, exhaustedContinue, result)
	assert.Contains(t, buf.String(), "dead letter handler failed")
}

func TestHandleExhausted_NoHandlerNoLogger(t *testing.T) {
	result := handleExhausted(
		context.Background(),
		"test",
		"hello",
		1,
		errors.New("fail"),
		nil,
		nil,
		nil,
		nil,
	)

	assert.Equal(t, exhaustedContinue, result)
}

// ==================== Consumer with PanicHandler ====================

func TestConsumer_WithPanicHandler(t *testing.T) {
	client := &mockGetClient{
		results: []mockGetResult{
			{data: "msg1", pos: 1},
		},
	}

	var panicVal atomic.Value

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("panic-queue", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			panic("test panic")
		})),
		WithMaxRetry(0),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithPanicHandler(func(v any) {
			panicVal.Store(v)
		}),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	// Wait for panic to be caught
	start := time.Now()
	for panicVal.Load() == nil {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout waiting for panic handler")
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.Equal(t, "test panic", panicVal.Load())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

// ==================== Consumer with HandlerTimeout ====================

func TestConsumer_WithHandlerTimeout(t *testing.T) {
	client := &mockGetClient{
		results: []mockGetResult{
			{data: "msg1", pos: 1},
		},
	}

	var consumed atomic.Int32

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("timeout-queue", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			consumed.Add(1)
			time.Sleep(200 * time.Millisecond)
			return ctx.Err()
		})),
		WithMaxRetry(0),
		WithHandlerTimeout(50*time.Millisecond),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	start := time.Now()
	for consumed.Load() < 1 {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout, consumed: %d", consumed.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

// ==================== Queue-level override with HTTPSQS client ====================

func TestConsumer_WithQueueHTTPSQSClient(t *testing.T) {
	client := &mockGetClient{}

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return nil
		}), WithQueueHTTPSQSClient(client)),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())
}

// ==================== Queue-level retry mode override ====================

func TestConsumer_WithQueueRetryMode(t *testing.T) {
	client := &mockGetClient{}

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return nil
		}), WithQueueRetryMode(RetryModeRequeue)),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())
}

// ==================== Queue-level backoff override ====================

func TestConsumer_WithQueueBackoff(t *testing.T) {
	client := &mockGetClient{}

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return nil
		}), WithQueueBackoff(&retry.FixedDelay{Wait: time.Millisecond})),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())
}

// ==================== Queue-level max retry override ====================

func TestConsumer_WithQueueMaxRetry(t *testing.T) {
	client := &mockGetClient{}

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return nil
		}), WithQueueMaxRetry(5)),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())
}

