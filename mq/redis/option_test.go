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
	"github.com/gomooth/pkg/mq/internal/types"
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
		{Queue: "q1", Handler: types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })},
		{Queue: "q2", Handler: types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })},
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
		WithConsumer("q1", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithConsumerLogger(logger),
	)

	err := consumer.Start(context.Background())
	require.NoError(t, err)

	// Register after start should return error
	err = consumer.Register("q2", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil }))
	assert.Error(t, err)
	assert.Equal(t, uint(1), consumer.Count())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

func TestConsumer_RegisterBeforeStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q1", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(1), consumer.Count())

	// Register before start should succeed
	err := consumer.Register("q2", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil }))
	assert.NoError(t, err)
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
	err := consumer.Register("", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil }))
	assert.NoError(t, err) // Register itself doesn't return error for empty queue, just logs
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
	err := consumer.Register("q1", nil)
	assert.NoError(t, err) // Register itself doesn't return error for nil handler, just logs
	assert.Contains(t, buf.String(), "handler must not be nil")
	assert.Equal(t, uint(0), consumer.Count())
}

// ==================== Consumer Shutdown before start ====================

func TestConsumer_ShutdownBeforeStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })),
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
		WithConsumer("q", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })),
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
		WithConsumer("requeue-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			n := handleAttempts.Add(1)
			if n < 2 {
				return errors.New("fail")
			}
			return nil
		})),
		WithMaxRetry(3),
		WithBackoff(&retry.FixedDelay{Wait: time.Millisecond}),
		WithRetryMode(types.RetryModeRequeue),
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
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return errors.New("always fail")
		}),
		0,
		&retry.FixedDelay{Wait: time.Millisecond},
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	// SetDeadLetterHandler should set the deadLetter field
	dl := typesDeadLetterFunc(func(ctx context.Context, msg types.Message, lastErr error) error {
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
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
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
	dl := typesDeadLetterFunc(func(ctx context.Context, msg types.Message, lastErr error) error {
		dlCalled.Add(1)
		return nil
	})
	strategy.SetDeadLetterHandler(dl)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), dlCalled.Load())
}

// ==================== Consumer with custom Redis config ====================

func TestConsumer_WithCustomRedisConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	opts := &redis.Options{Addr: mr.Addr()}
	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })),
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
		WithConsumer("panic-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
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

	err = producer.ProduceBatch(ctx, "q", [][]byte{[]byte("a"), []byte("b")})
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

	err = producer.ProduceBatch(context.Background(), "q", [][]byte{[]byte("a")})
	assert.Error(t, err)
}

// ==================== Consumer with HandlerTimeout ====================

func TestConsumer_WithHandlerTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var consumed atomic.Int32

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("timeout-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
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

// typesDeadLetterFunc 死信适配器，适配 types.DeadLetterHandler 接口
type typesDeadLetterFunc func(ctx context.Context, msg types.Message, lastErr error) error

func (f typesDeadLetterFunc) OnDeadLetter(ctx context.Context, msg types.Message, lastErr error) error {
	return f(ctx, msg, lastErr)
}
