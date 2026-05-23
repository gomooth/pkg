package queue

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSimpleRedis(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 使用短超时避免空 Pop 阻塞过久
	redisCnf := &RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 1 * time.Second,
	}
	q := NewSimpleRedis(redisCnf, "test")

	var consumed atomic.Int32

	// 生产者：快速推送30条消息
	go func() {
		for i := 0; i < 30; i++ {
			err := q.Push(ctx, fmt.Sprintf("testNo:%d", i))
			if err != nil {
				t.Logf("push %d error: %+v", i, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// 消费者：循环 Pop 直到收到30条或超时
	for consumed.Load() < 30 {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for messages, consumed: %d", consumed.Load())
		default:
		}

		str, err := q.Pop(ctx)
		if err != nil {
			t.Logf("pop error: %+v", err)
			continue
		}
		if len(str) > 0 {
			consumed.Add(1)
			t.Logf("pop queue: %s, total: %d", str, consumed.Load())
		}
	}

	assert.Equal(t, int32(30), consumed.Load())
}

func TestSimpleRedis_Pop_AtomicLua(t *testing.T) {
	mr := miniredis.RunT(t)
	q := NewSimpleRedis(&RedisQueueConfig{Addr: mr.Addr()}, "test_atomic")

	ctx := context.Background()

	// Push 一条消息
	err := q.Push(ctx, "hello")
	assert.NoError(t, err)

	// Pop 应该原子性地获取
	val, err := q.Pop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "hello", val)
}
