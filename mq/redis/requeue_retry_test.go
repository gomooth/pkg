package redis

import (
	"context"
	"errors"
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

func TestRequeueRetryStrategy_Success(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := miniredisClient(t, mr)

	strategy := newRequeueRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		}),
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"queue:",
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
}

func TestRequeueRetryStrategy_RequeueThenSuccess(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := miniredisClient(t, mr)

	var handleAttempts atomic.Int32
	strategy := newRequeueRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			n := handleAttempts.Add(1)
			if n < 3 {
				return errors.New("fail")
			}
			return nil
		}),
		5,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"queue:",
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	// 第一次处理失败 → 再入队
	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)

	// 验证消息被再入队到 Redis
	queueKey := "queue:test"
	val, err := client.LRange(context.Background(), queueKey, 0, -1).Result()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(val), 1, "message should be requeued")
}

func TestRequeueRetryStrategy_Exhausted(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := miniredisClient(t, mr)

	var failedCalled atomic.Int32
	strategy := newRequeueRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return errors.New("always fail")
		}),
		0, // maxRetry=0 means no retries, immediate exhaustion
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"queue:",
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.SetFailedHandler(func(ctx context.Context, msg types.Message, err error) {
		failedCalled.Add(1)
	})

	err := strategy.OnMessage(context.Background(), "test", []byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), failedCalled.Load())

	// 重试耗尽后不应再入队
	queueKey := "queue:test"
	val, err := client.LRange(context.Background(), queueKey, 0, -1).Result()
	require.NoError(t, err)
	assert.Equal(t, 0, len(val), "exhausted message should not be requeued")
}

func TestRequeueRetryStrategy_ContextCancel(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := miniredisClient(t, mr)

	strategy := newRequeueRetryStrategy(
		types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return errors.New("fail")
		}),
		100,
		&retry.FixedDelay{Wait: 10 * time.Millisecond},
		client,
		"queue:",
		logutil.NewSlogLogger(nilLogger()),
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := strategy.OnMessage(ctx, "test", []byte("hello"))
	// context 已取消，可能在 Handle 或退避等待中返回
	assert.True(t, err != nil, "should return error on context cancel")
}

// miniredisClient 创建连接到 miniredis 的 redis.Client
func miniredisClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}
