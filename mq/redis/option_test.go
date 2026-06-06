package redis

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/redis/go-redis/v9"
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
		{Queue: "q1", Handler: FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })},
		{Queue: "q2", Handler: FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })},
	}
	WithConsumers(regs...)(cfg)
	assert.Len(t, cfg.consumers, 2)
}

func TestWithConsumerLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &consumerConfig{}
	WithConsumerLogger(logger)(cfg)
	assert.Equal(t, logger, cfg.logger)
}

func TestWithConsumerRedisConfig(t *testing.T) {
	opts := &redis.Options{Addr: "localhost:6379", DB: 5}
	cfg := &consumerConfig{}
	WithConsumerRedisConfig(opts)(cfg)
	assert.Equal(t, opts, cfg.redisOptions)
}

func TestWithQueuePrefix(t *testing.T) {
	cfg := &consumerConfig{}
	WithQueuePrefix("myqueue:")(cfg)
	assert.Equal(t, "myqueue:", cfg.queuePrefix)
}

func TestWithProducerLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &producerConfig{}
	WithProducerLogger(logger)(cfg)
	assert.Equal(t, logger, cfg.logger)
}

func TestWithProducerRedisConfig(t *testing.T) {
	opts := &redis.Options{Addr: "localhost:6379", DB: 3}
	cfg := &producerConfig{}
	WithProducerRedisConfig(opts)(cfg)
	assert.Equal(t, opts, cfg.redisOptions)
}

// ==================== Register Test ====================

func TestConsumer_RegisterAfterStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithConsumerLogger(logger),
	)

	err := consumer.Start(context.Background())
	require.NoError(t, err)

	// Register after start should log error and skip
	consumer.Register("q2", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil }))
	assert.Contains(t, buf.String(), "cannot register after consumer started")
	assert.Equal(t, uint(1), consumer.Count())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

func TestConsumer_RegisterBeforeStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q1", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())

	// Register before start should succeed
	consumer.Register("q2", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil }))
	assert.Equal(t, uint(2), consumer.Count())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

func TestConsumer_RegisterEmptyQueueName(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	consumer := NewConsumer(mr.Addr(),
		WithConsumerLogger(logger),
	)

	// Register with empty queue name should log error
	consumer.Register("", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil }))
	assert.Contains(t, buf.String(), "queue name must not be empty")
	assert.Equal(t, uint(0), consumer.Count())
}

func TestConsumer_RegisterNilHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	consumer := NewConsumer(mr.Addr(),
		WithConsumerLogger(logger),
	)

	// Register with nil handler should log error
	consumer.Register("q1", nil)
	assert.Contains(t, buf.String(), "handler must not be nil")
	assert.Equal(t, uint(0), consumer.Count())
}

// ==================== Consumer Shutdown before start ====================

func TestConsumer_ShutdownBeforeStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	// Shutdown before start should not panic
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := consumer.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// ==================== Consumer Start after close ====================

func TestConsumer_StartAfterShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	err := consumer.Start(context.Background())
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)

	// Start after shutdown should fail
	err = consumer.Start(context.Background())
	assert.Error(t, err)
}

// ==================== Consumer with Requeue retry mode ====================

func TestConsumer_RequeueRetry(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var handleAttempts atomic.Int32

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("requeue-queue", FuncHandler(func(ctx context.Context, queue string, message []byte) error {
			n := handleAttempts.Add(1)
			if n < 2 {
				return errors.New("fail")
			}
			return nil
		})),
		WithMaxRetry(3),
		WithBackoff(&retry.FixedDelay{Wait: time.Millisecond}),
		WithRetryMode(RetryModeRequeue),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	client := miniredisClientForEngine(t, mr)
	err = client.LPush(ctx, "queue:requeue-queue", "msg1").Err()
	require.NoError(t, err)

	start := time.Now()
	for handleAttempts.Load() < 2 {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout, attempts: %d", handleAttempts.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

// ==================== SetDeadLetterHandler ====================

func TestSyncRetryStrategy_SetDeadLetterHandler(t *testing.T) {
	var dlCalled atomic.Int32

	strategy := newSyncRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, message []byte) error {
			return errors.New("always fail")
		}),
		0,
		&retry.FixedDelay{Wait: time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	// SetDeadLetterHandler should set the deadLetter field
	dl := deadLetterFunc(func(ctx context.Context, queue string, message []byte, lastErr error) error {
		dlCalled.Add(1)
		return nil
	})
	strategy.SetDeadLetterHandler(dl)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), dlCalled.Load())
}

func TestRequeueRetryStrategy_SetDeadLetterHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := miniredisClient(t, mr)

	var dlCalled atomic.Int32

	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, message []byte) error {
			return errors.New("always fail")
		}),
		0, // maxRetry=0, immediate exhaustion
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"queue:",
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	// SetDeadLetterHandler should set the deadLetter field
	dl := deadLetterFunc(func(ctx context.Context, queue string, message []byte, lastErr error) error {
		dlCalled.Add(1)
		return nil
	})
	strategy.SetDeadLetterHandler(dl)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), dlCalled.Load())
}

// ==================== handleExhausted with dead letter error ====================

func TestHandleExhausted_DeadLetterError(t *testing.T) {
	var buf bytes.Buffer
	logger := logutil.NewSlogLogger(slog.New(slog.NewTextHandler(&buf, nil)))

	result := handleExhausted(
		context.Background(),
		"test",
		[]byte("hello"),
		errors.New("fail"),
		deadLetterFunc(func(ctx context.Context, queue string, message []byte, lastErr error) error {
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
		[]byte("hello"),
		errors.New("fail"),
		nil,
		nil,
		nil,
		nil,
	)

	assert.Equal(t, exhaustedContinue, result)
}

// ==================== Consumer with custom Redis config ====================

func TestConsumer_WithCustomRedisConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	opts := &redis.Options{Addr: mr.Addr()}
	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", FuncHandler(func(ctx context.Context, queue string, message []byte) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithConsumerRedisConfig(opts),
	)

	err := consumer.Start(context.Background())
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

// ==================== Consumer with panic handler ====================

func TestConsumer_WithPanicHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var panicVal atomic.Value

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("panic-queue", FuncHandler(func(ctx context.Context, queue string, message []byte) error {
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

	client := miniredisClientForEngine(t, mr)
	err = client.LPush(ctx, "queue:panic-queue", "msg1").Err()
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

// ==================== Producer Shutdown before start ====================

func TestProducer_ShutdownBeforeStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := producer.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// ==================== Producer with custom Redis config ====================

func TestProducer_WithCustomRedisConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	opts := &redis.Options{Addr: mr.Addr()}
	producer := NewProducer(mr.Addr(), WithProducerRedisConfig(opts))

	err := producer.Start(context.Background())
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)
}

// ==================== Producer Start after close ====================

func TestProducer_StartAfterShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)

	err = producer.Start(context.Background())
	assert.Error(t, err)
}

// ==================== Producer Produce with cancelled context ====================

func TestProducer_ProduceCancelledContext(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = producer.Produce(ctx, "q", []byte("m"))
	assert.Error(t, err)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = producer.Shutdown(shutdownCtx)
}

// ==================== Producer ProduceBatch with cancelled context ====================

func TestProducer_ProduceBatchCancelledContext(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = producer.ProduceBatch(ctx, "q", []byte("a"), []byte("b"))
	assert.Error(t, err)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = producer.Shutdown(shutdownCtx)
}

// ==================== Producer ProduceBatch after shutdown ====================

func TestProducer_ProduceBatchAfterShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)

	err = producer.ProduceBatch(context.Background(), "q", []byte("a"))
	assert.Error(t, err)
}

// ==================== Consumer with HandlerTimeout ====================

func TestConsumer_WithHandlerTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var consumed atomic.Int32

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("timeout-queue", FuncHandler(func(ctx context.Context, queue string, message []byte) error {
			consumed.Add(1)
			// Simulate slow handler that will be cancelled by timeout
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

	client := miniredisClientForEngine(t, mr)
	err = client.LPush(ctx, "queue:timeout-queue", "msg1").Err()
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

