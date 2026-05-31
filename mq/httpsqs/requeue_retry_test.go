package httpsqs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/httpsqs/internal"
	"github.com/stretchr/testify/assert"
)

// mockHTTPSQSClient 模拟 httpsqs.IClient
type mockHTTPSQSClient struct {
	getResult string
	getPos    int64
	getErr    error
	putCalled atomic.Int32
	putErr    error
}

func (m *mockHTTPSQSClient) Get(_ context.Context, _ string) (string, int64, error) {
	return m.getResult, m.getPos, m.getErr
}

func (m *mockHTTPSQSClient) Put(_ context.Context, _ string, _ string) (int64, error) {
	m.putCalled.Add(1)
	return 1, m.putErr
}

func (m *mockHTTPSQSClient) Status(_ context.Context, _ string) (*httpsqs.Status, error) {
	return nil, nil
}

func (m *mockHTTPSQSClient) View(_ context.Context, _ string, _ int64) (string, error) {
	return "", nil
}

func (m *mockHTTPSQSClient) Reset(_ context.Context, _ string) error { return nil }

func (m *mockHTTPSQSClient) SetMaxQueue(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockHTTPSQSClient) SetSyncTime(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func TestRequeueRetryStrategy_Success(t *testing.T) {
	client := &mockHTTPSQSClient{}

	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return nil
		}),
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"test-queue",
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), client.putCalled.Load(), "success should not requeue")
}

func TestRequeueRetryStrategy_RequeueOnFailure(t *testing.T) {
	client := &mockHTTPSQSClient{}

	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("fail")
		}),
		5,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"test-queue",
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), client.putCalled.Load(), "failure should requeue")
}

func TestRequeueRetryStrategy_Exhausted(t *testing.T) {
	client := &mockHTTPSQSClient{}

	var failedCalled atomic.Int32
	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("always fail")
		}),
		0, // maxRetry=0 means no retries, immediate exhaustion
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"test-queue",
		internal.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.SetFailedHandler(func(ctx context.Context, queue string, data string, pos int64, err error) {
		failedCalled.Add(1)
	})

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), failedCalled.Load())
	assert.Equal(t, int32(0), client.putCalled.Load(), "exhausted message should not be requeued")
}

func TestRequeueRetryStrategy_ContextCancel(t *testing.T) {
	client := &mockHTTPSQSClient{}

	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("fail")
		}),
		100,
		&retry.FixedDelay{Wait: 10 * time.Millisecond},
		client,
		"test-queue",
		internal.NewSlogLogger(nilLogger()),
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := strategy.OnMessage(ctx, "test", "hello", 1)
	// context 已取消，可能在退避等待中返回
	assert.True(t, err != nil, "should return error on context cancel")
}

func TestRequeueRetryStrategy_PutFailure(t *testing.T) {
	client := &mockHTTPSQSClient{putErr: errors.New("put failed")}

	var failedCalled atomic.Int32
	strategy := newRequeueRetryStrategy(
		FuncHandler(func(ctx context.Context, queue string, data string, pos int64) error {
			return errors.New("handle failed")
		}),
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"test-queue",
		internal.NewSlogLogger(nilLogger()),
		nil,
	)
	strategy.SetFailedHandler(func(ctx context.Context, queue string, data string, pos int64, err error) {
		failedCalled.Add(1)
	})

	err := strategy.OnMessage(context.Background(), "test", "hello", 1)
	assert.NoError(t, err)
	// Put 失败后走 exhausted 逻辑
	assert.Equal(t, int32(1), failedCalled.Load())
}
