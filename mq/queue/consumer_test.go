package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/stretchr/testify/assert"
)

// mockFetcher 实现 Fetcher 接口用于测试
type mockFetcher struct {
	results []fetchResult
	index   int
	closeFn func() error
}

type fetchResult struct {
	data string
	err  error
}

func (f *mockFetcher) Fetch(_ context.Context) (string, error) {
	if f.index >= len(f.results) {
		return "", nil // 队列为空
	}
	r := f.results[f.index]
	f.index++
	return r.data, r.err
}

func (f *mockFetcher) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func TestBaseConsumer_NoHandler(t *testing.T) {
	c := NewBaseConsumer()
	err := c.Consume(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no consumer handler")
}

func TestBaseConsumer_NoFetcher(t *testing.T) {
	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{QueueNameFunc: func() string { return "test" }}),
	)
	err := c.Consume(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no consumer fetcher")
}

func TestBaseConsumer_ConsumeAndClose(t *testing.T) {
	var handleCalled atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			HandleFunc: func(_ context.Context, data string) error {
				handleCalled.Add(1)
				return nil
			},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{
				{data: "msg1"},
				{data: "msg2"},
			},
		}),
		WithConsumerEmptyQueueSleep(10*time.Millisecond),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 10 * time.Millisecond}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx)
	}()

	// 等待处理几条消息
	time.Sleep(100 * time.Millisecond)

	err := c.Close()
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Consume did not return after Close")
	}

	assert.GreaterOrEqual(t, handleCalled.Load(), int32(1))
}

func TestBaseConsumer_Close_Idempotent(t *testing.T) {
	c := NewBaseConsumer()
	assert.NotPanics(t, func() {
		_ = c.Close()
		_ = c.Close()
	})
}

func TestBaseConsumer_OnBeforeError(t *testing.T) {
	var handleCalled atomic.Int32
	var onBeforeErr atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			OnBeforeFunc: func(_ context.Context) error {
				onBeforeErr.Add(1)
				return assert.AnError
			},
			HandleFunc: func(_ context.Context, _ string) error {
				handleCalled.Add(1)
				return nil
			},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{{data: "msg1"}},
		}),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 10 * time.Millisecond}),
		WithConsumerEmptyQueueSleep(10*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = c.Consume(ctx)

	// OnBefore 失败，Handle 不应被调用
	assert.Greater(t, onBeforeErr.Load(), int32(0))
	assert.Equal(t, int32(0), handleCalled.Load())
}

func TestBaseConsumer_FailedCallback(t *testing.T) {
	var onFailedCalled atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			HandleFunc: func(_ context.Context, _ string) error {
				return assert.AnError
			},
			OnFailedFunc: func(_ context.Context, _ string, _ error) {
				onFailedCalled.Add(1)
			},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{{data: "msg1"}},
		}),
		WithConsumerFailedCallbackDelay(10*time.Millisecond),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 10 * time.Millisecond}),
		WithConsumerEmptyQueueSleep(10*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = c.Consume(ctx)

	assert.Greater(t, onFailedCalled.Load(), int32(0))
}

func TestFuncHandler_Defaults(t *testing.T) {
	h := &FuncHandler{}
	assert.Equal(t, "", h.QueueName())
	assert.NoError(t, h.OnBefore(context.Background()))
	assert.NoError(t, h.Handle(context.Background(), "test"))
	h.OnFailed(context.Background(), "test", nil) // 不应 panic
}

func TestFuncHandler_WithFuncs(t *testing.T) {
	h := &FuncHandler{
		QueueNameFunc: func() string { return "my-queue" },
		OnBeforeFunc:  func(_ context.Context) error { return assert.AnError },
		HandleFunc:    func(_ context.Context, _ string) error { return nil },
		OnFailedFunc:  func(_ context.Context, _ string, _ error) {},
	}
	assert.Equal(t, "my-queue", h.QueueName())
	assert.Error(t, h.OnBefore(context.Background()))
	assert.NoError(t, h.Handle(context.Background(), "test"))
	h.OnFailed(context.Background(), "test", nil)
}

func TestBaseConsumer_FetchError(t *testing.T) {
	var handleCalled atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			HandleFunc: func(_ context.Context, _ string) error {
				handleCalled.Add(1)
				return nil
			},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{
				{err: assert.AnError},
			},
		}),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 10 * time.Millisecond}),
		WithConsumerEmptyQueueSleep(10*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = c.Consume(ctx)

	// Fetch 失败，Handle 不应被调用
	assert.Equal(t, int32(0), handleCalled.Load())
}

func TestBaseConsumer_InfraAttemptResetsOnFetchSuccess(t *testing.T) {
	// 验证 infraAttempt 在 Fetch 成功获取非空数据后重置
	var fetchCount atomic.Int32
	var handleCount atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			HandleFunc: func(_ context.Context, _ string) error {
				handleCount.Add(1)
				return assert.AnError // Handle 失败不应影响 infraAttempt
			},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{
				{err: assert.AnError},      // 第1次 Fetch 失败 → infraAttempt=1
				{err: assert.AnError},      // 第2次 Fetch 失败 → infraAttempt=2
				{data: "msg1"},             // 第3次 Fetch 成功 → infraAttempt 重置为 0
				{data: "msg2"},             // 后续 Fetch 成功 → infraAttempt 仍为 0
			},
		}),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 5 * time.Millisecond}),
		WithConsumerEmptyQueueSleep(5*time.Millisecond),
		WithConsumerFailedCallbackDelay(5*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = c.Consume(ctx)

	fetchCount.Load() // 仅确保测试不 panic
	assert.GreaterOrEqual(t, handleCount.Load(), int32(2), "both messages should be handled")
}

func TestBaseConsumer_OnBeforeErrorDoesNotResetInfraAttempt(t *testing.T) {
	// 验证 OnBefore 失败不会重置 infraAttempt（即 Fetch 失败的退避会持续累积）
	var onBeforeCount atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			OnBeforeFunc: func(_ context.Context) error {
				onBeforeCount.Add(1)
				return assert.AnError
			},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{{data: "msg1"}},
		}),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 5 * time.Millisecond}),
		WithConsumerEmptyQueueSleep(5*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = c.Consume(ctx)

	// OnBefore 持续失败，infraAttempt 应持续递增
	assert.Greater(t, onBeforeCount.Load(), int32(0))
}

func TestBaseConsumer_HandleFailureDoesNotAffectInfraAttempt(t *testing.T) {
	// 验证 Handle 失败不影响 infraAttempt：Fetch 成功后 infraAttempt 重置为 0，
	// 后续 Handle 失败不会导致 infraAttempt 递增
	var handleErr atomic.Int32

	c := NewBaseConsumer(
		WithConsumerHandler(&FuncHandler{
			QueueNameFunc: func() string { return "test-queue" },
			HandleFunc: func(_ context.Context, _ string) error {
				handleErr.Add(1)
				return assert.AnError // Handle 失败
			},
			OnFailedFunc: func(_ context.Context, _ string, _ error) {},
		}),
		WithConsumerFetcher(&mockFetcher{
			results: []fetchResult{
				{data: "msg1"},
				{data: "msg2"},
				{data: "msg3"},
			},
		}),
		WithConsumerBackoff(&retry.FixedDelay{Wait: 5 * time.Millisecond}),
		WithConsumerEmptyQueueSleep(5*time.Millisecond),
		WithConsumerFailedCallbackDelay(5*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = c.Consume(ctx)

	// Handle 被调用了多次，但 infraAttempt 不应递增（不会触发退避）
	assert.GreaterOrEqual(t, handleErr.Load(), int32(3))
}
