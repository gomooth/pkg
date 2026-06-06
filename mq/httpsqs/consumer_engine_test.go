package httpsqs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== mock client ====================

// mockGetClient 支持顺序返回多条消息的 mock 客户端
type mockGetClient struct {
	results []mockGetResult
	index   atomic.Int64
}

type mockGetResult struct {
	data string
	pos  int64
	err  error
}

func (m *mockGetClient) Get(_ context.Context, _ string) (string, int64, error) {
	idx := int(m.index.Add(1)) - 1
	if idx >= len(m.results) {
		return "", -1, nil // 队列为空
	}
	r := m.results[idx]
	return r.data, r.pos, r.err
}

func (m *mockGetClient) Put(_ context.Context, _ string, _ string) (int64, error) {
	return 1, nil
}

func (m *mockGetClient) Status(_ context.Context, _ string) (*httpsqs.Status, error) {
	return nil, nil
}

func (m *mockGetClient) View(_ context.Context, _ string, _ int64) (string, error) {
	return "", nil
}

func (m *mockGetClient) Reset(_ context.Context, _ string) error { return nil }

func (m *mockGetClient) SetMaxQueue(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockGetClient) SetSyncTime(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

// ==================== tests ====================

func TestNewConsumer(t *testing.T) {
	client := &mockGetClient{}
	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("test-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		})),
	)
	assert.NotNil(t, consumer)
	assert.Equal(t, uint(1), consumer.Count())
}

func TestConsumer_StartShutdown(t *testing.T) {
	client := &mockGetClient{
		results: []mockGetResult{
			{data: "msg1", pos: 1},
			{data: "msg2", pos: 2},
			{data: "msg3", pos: 3},
		},
	}

	var consumed atomic.Int32

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
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

func TestConsumer_NoClient(t *testing.T) {
	consumer := NewConsumer(
		WithConsumer("test-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		})),
	)
	err := consumer.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "httpsqs client is required")
}

func TestConsumer_NoRegistrations(t *testing.T) {
	client := &mockGetClient{}
	consumer := NewConsumer(WithHTTPSQSClient(client))
	err := consumer.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no consumers registered")
}

func TestConsumer_DoubleStart(t *testing.T) {
	client := &mockGetClient{}
	consumer := NewConsumer(
		WithHTTPSQSClient(client),
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
	client := &mockGetClient{}
	consumer := NewConsumer(
		WithHTTPSQSClient(client),
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
	client := &mockGetClient{
		results: []mockGetResult{
			{data: "msg1", pos: 1},
		},
	}

	var handleAttempts atomic.Int32

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("retry-queue", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			n := handleAttempts.Add(1)
			if n < 3 {
				return errors.New("fail")
			}
			return nil
		})),
		WithMaxRetry(5),
		WithBackoff(&retry.FixedDelay{Wait: time.Millisecond}),
		WithRetryMode(RetryModeSync),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
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
	client := &mockGetClient{
		results: []mockGetResult{
			{data: "msg1", pos: 1},
		},
	}

	var failedCalled atomic.Int32

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
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

func TestConsumer_MultipleQueues(t *testing.T) {
	client := &mockGetClient{
		results: []mockGetResult{
			{data: "msg-q1", pos: 1},
			{data: "msg-q2", pos: 1},
		},
	}

	var q1Consumed, q2Consumed atomic.Int32

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("queue1", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			q1Consumed.Add(1)
			return nil
		})),
		WithConsumer("queue2", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			q2Consumed.Add(1)
			return nil
		})),
		WithEmptyQueueSleep(50*time.Millisecond),
		WithMaxRetry(0),
	)

	ctx := context.Background()
	err := consumer.Start(ctx)
	require.NoError(t, err)

	assert.Equal(t, uint(2), consumer.Count())

	// 等待消费
	start := time.Now()
	for q1Consumed.Load()+q2Consumed.Load() < 2 {
		if time.Since(start) > 3*time.Second {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = consumer.Shutdown(shutdownCtx)
}

func TestConsumer_QueueLevelOverride(t *testing.T) {
	client := &mockGetClient{}

	var q2FailedCalled atomic.Int32

	consumer := NewConsumer(
		WithHTTPSQSClient(client),
		WithConsumer("q1", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		})),
		WithConsumer("q2", types.FuncHandler(func(ctx context.Context, msg types.Message) error {
			return nil
		}), types.WithQueueFailedHandler(func(ctx context.Context, msg types.Message, err error) {
			q2FailedCalled.Add(1)
		})),
		WithEmptyQueueSleep(50*time.Millisecond),
	)

	assert.Equal(t, uint(2), consumer.Count())
	// Queue-level override 应该不影响全局配置
	assert.Equal(t, int32(0), q2FailedCalled.Load())
}

func TestConsumer_ShutdownIdempotent(t *testing.T) {
	client := &mockGetClient{}
	consumer := NewConsumer(WithHTTPSQSClient(client))

	assert.NotPanics(t, func() {
		_ = consumer.Shutdown(context.Background())
		_ = consumer.Shutdown(context.Background())
	})
}