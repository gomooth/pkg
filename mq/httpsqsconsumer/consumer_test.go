package httpsqsconsumer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/stretchr/testify/assert"
)

// mockIClient 模拟 httpsqs.IClient
type mockIClient struct {
	getResult string
	getPos    int64
	getErr    error
}

func (m *mockIClient) Get(_ context.Context, _ string) (string, int64, error) {
	return m.getResult, m.getPos, m.getErr
}

func (m *mockIClient) Put(_ context.Context, _ string, _ string) (int64, error) {
	return 1, nil
}

func (m *mockIClient) Status(_ context.Context, _ string) (*httpsqs.Status, error) {
	return nil, nil
}

func (m *mockIClient) View(_ context.Context, _ string, _ int64) (string, error) {
	return "", nil
}

func (m *mockIClient) Reset(_ context.Context, _ string) error { return nil }

func (m *mockIClient) SetMaxQueue(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockIClient) SetSyncTime(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

// mockHandler 模拟 IHandler
type mockHandler struct {
	client      *mockIClient
	queueName   string
	onBeforeErr error
	handleErr   error

	onFailedCalled atomic.Int32
}

func (m *mockHandler) QueueName() string { return m.queueName }

func (m *mockHandler) GetClient() (httpsqs.IClient, error) {
	return m.client, nil
}

func (m *mockHandler) OnBefore(_ context.Context) error { return m.onBeforeErr }

func (m *mockHandler) Handle(_ context.Context, _ string, _ int64) error { return m.handleErr }

func (m *mockHandler) OnFailed(_ context.Context, _ string, _ error) {
	m.onFailedCalled.Add(1)
}

func TestConsumer_Close_StopsConsume(t *testing.T) {
	client := &mockIClient{getResult: "data", getPos: 1}
	handler := &mockHandler{client: client, queueName: "test-queue"}

	c := New(
		WithHandler(handler),
		WithEmptyQueueSleep(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	err := c.Close()
	assert.NoError(t, err)

	select {
	case err := <-done:
		assert.Error(t, err, "Consume should return with context cancelled error")
	case <-time.After(2 * time.Second):
		t.Fatal("Consume did not return after Close")
	}
}

func TestConsumer_FailedGoroutine_ExitsOnClose(t *testing.T) {
	client := &mockIClient{getResult: "data", getPos: 1}
	handler := &mockHandler{
		client:    client,
		queueName: "test-queue",
		handleErr: assert.AnError,
	}

	c := New(
		WithHandler(handler),
		WithFailedCallbackDelay(10*time.Millisecond),
		WithEmptyQueueSleep(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	err := c.Close()
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Consume did not return after Close")
	}
}

func TestConsumer_Close_Idempotent(t *testing.T) {
	c := New()

	assert.NotPanics(t, func() {
		_ = c.Close()
		_ = c.Close()
	})
}

func TestConsumer_NoHandler(t *testing.T) {
	c := New()
	err := c.Consume(context.Background())
	assert.Error(t, err)
}
