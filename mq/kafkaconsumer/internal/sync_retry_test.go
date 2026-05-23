package internal

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/stretchr/testify/assert"
)

func TestSyncRetry_MaxTotalTimeout_StopsRetries(t *testing.T) {
	var handlerCalls atomic.Int32

	conf := &groupHandlerConf{
		Handler: func(_ context.Context, _ string, _ []byte) error {
			handlerCalls.Add(1)
			return errors.New("always fail")
		},
	}

	s := newSyncRetryStrategy("test-cg", conf, &retry.FixedDelay{Wait: 100 * time.Millisecond}, 250*time.Millisecond)
	s.SetLogger(newSlogLogger(nil))

	session := &mockSession{ctx: context.Background()}
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}

	s.OnMessage(context.Background(), session, msg)

	calls := handlerCalls.Load()
	assert.LessOrEqual(t, calls, int32(3), "should stop retries due to total timeout")
	assert.GreaterOrEqual(t, calls, int32(1), "should have at least one attempt")
}

func TestSyncRetry_NoMaxTotalTimeout_ReachesMaxRetry(t *testing.T) {
	var handlerCalls atomic.Int32

	conf := &groupHandlerConf{
		Handler: func(_ context.Context, _ string, _ []byte) error {
			handlerCalls.Add(1)
			return errors.New("always fail")
		},
	}

	s := newSyncRetryStrategy("test-cg", conf, &retry.FixedDelay{Wait: time.Millisecond}, 0)
	s.maxRetry = 3
	s.SetLogger(newSlogLogger(nil))

	session := &mockSession{ctx: context.Background()}
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}

	s.OnMessage(context.Background(), session, msg)

	assert.Equal(t, int32(4), handlerCalls.Load(), "should attempt maxRetry+1 times when no total timeout")
}

func TestSyncRetry_HandlerTimeout(t *testing.T) {
	var handlerCalls atomic.Int32

	conf := &groupHandlerConf{
		Handler: func(ctx context.Context, _ string, _ []byte) error {
			handlerCalls.Add(1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		},
		HandlerTimeout: 50 * time.Millisecond,
	}

	s := newSyncRetryStrategy("test-cg", conf, &retry.FixedDelay{Wait: time.Millisecond}, 0)
	s.maxRetry = 1
	s.SetLogger(newSlogLogger(nil))

	// 超时 context 由 ConsumeClaim 层注入，这里直接传入带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	session := &mockSession{ctx: context.Background()}
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}

	start := time.Now()
	s.OnMessage(ctx, session, msg)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second, "should return quickly on handler timeout")
	assert.Equal(t, int32(2), handlerCalls.Load(), "should attempt maxRetry+1 times (initial + 1 retry)")
}
