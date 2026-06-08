package redis

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumer(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("test-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		})),
	)
	assert.NotNil(t, consumer)
	assert.Equal(t, uint(1), consumer.Count())
}

func TestConsumer_StartShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var consumed atomic.Int32

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("test-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			consumed.Add(1)
			return nil
		})),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithMaxRetry(0),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	// 推送消息
	client := miniredisClientForEngine(t, mr)
	err = client.LPush(ctx, "queue:test-queue", "msg1", "msg2", "msg3").Err()
	require.NoError(t, err)

	// 等待消费
	start := time.Now()
	for consumed.Load() < 3 {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout waiting for consumption, consumed: %d", consumed.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = consumer.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestConsumer_NoRegistrations(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr())
	err := consumer.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no consumers registered")
}

func TestConsumer_DoubleStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		})),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	err := consumer.Start(context.Background())
	require.NoError(t, err)

	err = consumer.Start(context.Background())
	assert.NoError(t, err) // 已运行时重复 Start 返回 nil

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

func TestConsumer_HealthCheck(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("q", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		})),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	// 未启动时不健康
	err := consumer.HealthCheck(context.Background())
	assert.Error(t, err)

	_ = consumer.Start(context.Background())

	// 启动后健康
	err = consumer.HealthCheck(context.Background())
	assert.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)

	// 关闭后不健康
	err = consumer.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestConsumer_SyncRetry(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var handleAttempts atomic.Int32

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("retry-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			n := handleAttempts.Add(1)
			if n < 3 {
				return errors.New("fail")
			}
			return nil
		})),
		WithMaxRetry(5),
		WithBackoff(&retry.FixedDelay{Wait: time.Millisecond}),
		WithRetryMode(types.RetryModeSync),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	client := miniredisClientForEngine(t, mr)
	err = client.LPush(ctx, "queue:retry-queue", "msg1").Err()
	require.NoError(t, err)

	start := time.Now()
	for handleAttempts.Load() < 3 {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout, attempts: %d", handleAttempts.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)

	assert.GreaterOrEqual(t, handleAttempts.Load(), int32(3))
}

func TestConsumer_FailedHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	var failedCalled atomic.Int32

	consumer := NewConsumer(mr.Addr(),
		WithConsumer("fail-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return errors.New("always fail")
		})),
		WithMaxRetry(1),
		WithBackoff(&retry.FixedDelay{Wait: time.Millisecond}),
		WithFailedHandler(func(ctx context.Context, msg types.Message, err error) {
			failedCalled.Add(1)
		}),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	client := miniredisClientForEngine(t, mr)
	err = client.LPush(ctx, "queue:fail-queue", "msg1").Err()
	require.NoError(t, err)

	start := time.Now()
	for failedCalled.Load() < 1 {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout, failedCalled: %d", failedCalled.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

// miniredisClientForEngine 创建连接到 miniredis 的 redis.Client（消费者测试用）
func miniredisClientForEngine(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}
