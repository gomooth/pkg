package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProducer(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	assert.NotNil(t, producer)
}

func TestProducer_StartShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())

	err := producer.Start(context.Background())
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = producer.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestProducer_Produce(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	ctx := context.Background()
	err = producer.Produce(ctx, "test-queue", []byte("hello"))
	assert.NoError(t, err)

	// 验证消息在 Redis 中
	client := miniredisProducerClient(t, mr)
	val, err := client.LRange(ctx, "queue:test-queue", 0, -1).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, len(val))
	assert.Equal(t, "hello", val[0])

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)
}

func TestProducer_ProduceBatch(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	ctx := context.Background()
	err = producer.ProduceBatch(ctx, "batch-queue", []byte("a"), []byte("b"), []byte("c"))
	assert.NoError(t, err)

	client := miniredisProducerClient(t, mr)
	val, err := client.LRange(ctx, "queue:batch-queue", 0, -1).Result()
	require.NoError(t, err)
	assert.Equal(t, 3, len(val))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)
}

func TestProducer_ProduceBeforeStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Produce(context.Background(), "q", []byte("m"))
	assert.Error(t, err)
}

func TestProducer_ProduceAfterShutdown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	_ = producer.Start(context.Background())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)

	err := producer.Produce(context.Background(), "q", []byte("m"))
	assert.Error(t, err)
}

func TestProducer_CustomPrefix(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr(), WithProducerQueuePrefix("myq:"))
	err := producer.Start(context.Background())
	require.NoError(t, err)

	ctx := context.Background()
	err = producer.Produce(ctx, "test", []byte("hello"))
	assert.NoError(t, err)

	client := miniredisProducerClient(t, mr)
	val, err := client.LRange(ctx, "myq:test", 0, -1).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, len(val))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)
}

func TestProducer_ProduceBatchEmpty(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	_ = producer.Start(context.Background())

	err := producer.ProduceBatch(context.Background(), "q")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)
}

func TestProducer_DoubleStart(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	producer := NewProducer(mr.Addr())
	err := producer.Start(context.Background())
	require.NoError(t, err)

	err = producer.Start(context.Background())
	assert.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = producer.Shutdown(shutdownCtx)
}

// miniredisProducerClient 创建连接到 miniredis 的 redis.Client（生产者测试用）
func miniredisProducerClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}
