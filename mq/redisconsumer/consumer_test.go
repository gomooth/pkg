package redisconsumer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomooth/pkg/mq/queue"
)

func TestNew(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
}

func TestConsumer_Consume_Success(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &queue.RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 100 * time.Millisecond,
	}

	queueName := "test_success"
	testMessages := []string{"msg1", "msg2", "msg3"}

	var consumed atomic.Int32
	var handlerCalled atomic.Int32
	var mu sync.Mutex
	receivedMessages := make([]string, 0, len(testMessages)*2)

	producerQueue := queue.NewSimpleRedis(config, queueName)
	ctx := context.Background()

	for _, msg := range testMessages {
		err := producerQueue.Push(ctx, msg)
		require.NoError(t, err)
	}
	err := producerQueue.Close()
	require.NoError(t, err)

	consumer := New(
		WithHandler(config, queueName, func(val string) error {
			handlerCalled.Add(1)
			mu.Lock()
			receivedMessages = append(receivedMessages, val)
			mu.Unlock()
			consumed.Add(1)
			return nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = consumer.Consume(ctx)
	}()

	start := time.Now()
	for consumed.Load() < int32(len(testMessages)) {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout waiting for messages, consumed: %d, expected: %d",
				consumed.Load(), len(testMessages))
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	assert.GreaterOrEqual(t, consumed.Load(), int32(len(testMessages)))
	assert.GreaterOrEqual(t, handlerCalled.Load(), int32(len(testMessages)))

	for _, msg := range testMessages {
		mu.Lock()
		found := false
		for _, received := range receivedMessages {
			if received == msg {
				found = true
				break
			}
		}
		mu.Unlock()
		assert.True(t, found, "message %s was not received", msg)
	}
}

func TestConsumer_Consume_HandlerFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &queue.RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 100 * time.Millisecond,
	}

	queueName := "test_failure"
	testMessages := []string{"msg1", "msg2", "msg3"}

	var handlerCalled atomic.Int32
	var failedHandlerCalled atomic.Int32
	var mu sync.Mutex
	receivedMessages := make([]string, 0, len(testMessages)*2)
	failedMessages := make([]string, 0, len(testMessages)*2)

	expectedError := errors.New("handler failed")

	producerQueue := queue.NewSimpleRedis(config, queueName)
	ctx := context.Background()

	for _, msg := range testMessages {
		err := producerQueue.Push(ctx, msg)
		require.NoError(t, err)
	}
	err := producerQueue.Close()
	require.NoError(t, err)

	consumer := New(
		WithHandler(config, queueName, func(val string) error {
			handlerCalled.Add(1)
			mu.Lock()
			receivedMessages = append(receivedMessages, val)
			mu.Unlock()
			return expectedError
		}),
		WithFailedHandler(func(val string, err error) {
			failedHandlerCalled.Add(1)
			mu.Lock()
			failedMessages = append(failedMessages, val)
			mu.Unlock()
			assert.Equal(t, expectedError, err)
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = consumer.Consume(ctx)
	}()

	start := time.Now()
	for handlerCalled.Load() < int32(len(testMessages)) ||
		failedHandlerCalled.Load() < int32(len(testMessages)) {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout waiting for processing, handlerCalled: %d, failedHandlerCalled: %d, expected: %d",
				handlerCalled.Load(), failedHandlerCalled.Load(), len(testMessages))
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	assert.GreaterOrEqual(t, handlerCalled.Load(), int32(len(testMessages)))
	assert.GreaterOrEqual(t, failedHandlerCalled.Load(), int32(len(testMessages)))
}

func TestConsumer_Consume_ContextCancel(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &queue.RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 100 * time.Millisecond,
	}

	queueName := "test_cancel"
	testMessages := []string{"msg1", "msg2", "msg3"}

	var consumed atomic.Int32

	producerQueue := queue.NewSimpleRedis(config, queueName)
	ctx := context.Background()

	for _, msg := range testMessages {
		err := producerQueue.Push(ctx, msg)
		require.NoError(t, err)
	}
	err := producerQueue.Close()
	require.NoError(t, err)

	consumer := New(
		WithHandler(config, queueName, func(val string) error {
			consumed.Add(1)
			time.Sleep(50 * time.Millisecond)
			return nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	consumeErr := consumer.Consume(ctx)

	assert.Error(t, consumeErr)
	assert.ErrorIs(t, consumeErr, context.DeadlineExceeded)
	assert.Greater(t, consumed.Load(), int32(0))
}

func TestConsumer_Consume_EmptyQueue(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &queue.RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 100 * time.Millisecond,
	}

	queueName := "test_empty"
	var handlerCalled atomic.Int32

	consumer := New(
		WithHandler(config, queueName, func(val string) error {
			handlerCalled.Add(1)
			return nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	consumeErr := consumer.Consume(ctx)

	assert.Error(t, consumeErr)
	assert.ErrorIs(t, consumeErr, context.DeadlineExceeded)
	assert.Equal(t, int32(0), handlerCalled.Load())
}

func TestConsumer_Consume_DynamicMessages(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &queue.RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 100 * time.Millisecond,
	}

	queueName := "test_dynamic"
	var consumed atomic.Int32
	var mu sync.Mutex
	receivedMessages := make([]string, 0, 20)

	producerQueue := queue.NewSimpleRedis(config, queueName)
	ctx := context.Background()

	consumer := New(
		WithHandler(config, queueName, func(val string) error {
			mu.Lock()
			receivedMessages = append(receivedMessages, val)
			mu.Unlock()
			consumed.Add(1)
			return nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = consumer.Consume(ctx)
	}()

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("dynamic_msg_%d", i)
		err := producerQueue.Push(ctx, msg)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}

	err := producerQueue.Close()
	require.NoError(t, err)

	start := time.Now()
	for consumed.Load() < 10 {
		if time.Since(start) > 3*time.Second {
			t.Fatalf("timeout waiting for messages, consumed: %d, expected: 10", consumed.Load())
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	assert.GreaterOrEqual(t, consumed.Load(), int32(10))

	for i := 0; i < 10; i++ {
		expectedMsg := fmt.Sprintf("dynamic_msg_%d", i)
		mu.Lock()
		found := false
		for _, received := range receivedMessages {
			if received == expectedMsg {
				found = true
				break
			}
		}
		mu.Unlock()
		assert.True(t, found, "message %s was not received", expectedMsg)
	}
}

func TestConsumer_Close_StopsConsume(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &queue.RedisQueueConfig{
		Addr:    mr.Addr(),
		Timeout: 50 * time.Millisecond,
	}

	queueName := "test_close_stop"

	producerQueue := queue.NewSimpleRedis(config, queueName)
	err := producerQueue.Push(context.Background(), "trigger")
	require.NoError(t, err)
	err = producerQueue.Close()
	require.NoError(t, err)

	consumer := New(
		WithHandler(config, queueName, func(val string) error {
			return nil
		}),
	)

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- consumer.Consume(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	err = consumer.Close()
	assert.NoError(t, err)

	select {
	case err := <-done:
		assert.Error(t, err, "Consume should return with context cancelled error")
	case <-time.After(3 * time.Second):
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
