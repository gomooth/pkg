package kafka

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
)

type mockConsumerGroupSession struct {
	ctx   context.Context
	marks []*sarama.ConsumerMessage
}

func newMockSession() *mockConsumerGroupSession {
	return &mockConsumerGroupSession{ctx: context.Background()}
}
func (m *mockConsumerGroupSession) Claims() map[string][]int32               { return nil }
func (m *mockConsumerGroupSession) MemberID() string                         { return "test-member" }
func (m *mockConsumerGroupSession) GenerationID() int32                      { return 1 }
func (m *mockConsumerGroupSession) MarkOffset(string, int32, int64, string)  {}
func (m *mockConsumerGroupSession) Commit()                                  {}
func (m *mockConsumerGroupSession) ResetOffset(string, int32, int64, string) {}
func (m *mockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	m.marks = append(m.marks, msg)
}
func (m *mockConsumerGroupSession) Context() context.Context { return m.ctx }
func (m *mockConsumerGroupSession) Close()                   {}

func TestSyncRetry_Success(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	if len(session.marks) != 1 {
		t.Errorf("expected 1 marked message, got %d", len(session.marks))
	}
}

func TestSyncRetry_RetryThenSuccess(t *testing.T) {
	attempt := 0
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		attempt++
		if attempt < 3 {
			return errors.New("fail")
		}
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	if len(session.marks) != 1 {
		t.Errorf("expected 1 marked message after retry success, got %d", len(session.marks))
	}
}

func TestSyncRetry_Exhausted(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return errors.New("always fail")
	})
	strategy := newSyncRetryStrategy("test-group", handler, 2,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	// 重试耗尽且无 DeadLetterHandler，应标记消息（exhaustedHandled）
	if len(session.marks) != 1 {
		t.Errorf("expected 1 marked message after exhausted, got %d", len(session.marks))
	}
}

func TestSyncRetry_SetFailedHandler(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)

	strategy.SetFailedHandler(types.FailedHandlerFunc(func(ctx context.Context, msg types.Message, err error) {
	}))
	assert.NotNil(t, strategy.failedHandler)
}

func TestSyncRetry_SetDeadLetterHandler(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)

	dl := &testDeadLetterHandler{}
	strategy.SetDeadLetterHandler(dl)
	assert.NotNil(t, strategy.deadLetter)
}

func TestSyncRetry_SetSession(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	// SetSession is a no-op for sync strategy
	strategy.SetSession(newMockSession())
}

func TestSyncRetry_ClearSession(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	// ClearSession is a no-op for sync strategy
	strategy.ClearSession()
}

func TestSyncRetry_OnShutdown(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return nil
	})
	strategy := newSyncRetryStrategy("test-group", handler, 3,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	// OnShutdown is a no-op for sync strategy
	strategy.OnShutdown(context.Background())
}

func TestSyncRetry_ContextCancelled(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return errors.New("fail")
	})
	strategy := newSyncRetryStrategy("test-group", handler, 10,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(ctx, session, msg)
	// Should return early due to cancelled context
}

func TestSyncRetry_TotalTimeoutExceeded(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		time.Sleep(50 * time.Millisecond)
		return errors.New("fail")
	})
	strategy := newSyncRetryStrategy("test-group", handler, 100,
		&retry.ExponentialDelay{Base: 10 * time.Millisecond, Max: 100 * time.Millisecond},
		100*time.Millisecond, // total timeout
		nil, nil,
	)

	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	// Should stop retrying after total timeout exceeded
}

func TestSyncRetry_ExhaustedWithDeadLetter(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return errors.New("always fail")
	})

	dl := &testDeadLetterHandler{}
	strategy := newSyncRetryStrategy("test-group", handler, 1,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	strategy.SetDeadLetterHandler(dl)

	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	assert.True(t, dl.called.Load())
	// Dead letter handler succeeded -> exhaustedHandled -> MarkMessage
	assert.Len(t, session.marks, 1)
}

func TestSyncRetry_ExhaustedWithDeadLetterFail(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return errors.New("always fail")
	})

	dl := &failingDeadLetterHandler{}
	strategy := newSyncRetryStrategy("test-group", handler, 1,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	strategy.SetDeadLetterHandler(dl)

	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	// Dead letter handler failed -> exhaustedFailed -> no MarkMessage
	assert.Len(t, session.marks, 0)
}

func TestSyncRetry_ExhaustedWithFailedHandler(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return errors.New("always fail")
	})

	fhCalled := false
	fh := types.FailedHandlerFunc(func(ctx context.Context, msg types.Message, err error) {
		fhCalled = true
	})
	strategy := newSyncRetryStrategy("test-group", handler, 1,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, nil, nil,
	)
	strategy.SetFailedHandler(fh)

	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	assert.True(t, fhCalled)
	// Failed handler called -> exhaustedHandled -> MarkMessage
	assert.Len(t, session.marks, 1)
}

func TestSyncRetry_ExhaustedWithLogger(t *testing.T) {
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error {
		return errors.New("always fail")
	})

	var buf bytes.Buffer
	logger := logutil.NewSlogLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	strategy := newSyncRetryStrategy("test-group", handler, 1,
		&retry.ExponentialDelay{Base: time.Millisecond, Max: time.Second},
		0, logger, nil,
	)
	strategy.SetFailedHandler(DefaultFailedHandlerFunc(logger))

	session := newMockSession()
	msg := &sarama.ConsumerMessage{Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello")}
	strategy.OnMessage(context.Background(), session, msg)
	// failedHandler should have logged the error
	assert.Contains(t, buf.String(), "message consume failed")
}

// ==================== handleExhausted 单独测试 ====================

func TestHandleExhausted_WithDeadLetter_Success(t *testing.T) {
	dl := &testDeadLetterHandler{}
	result := handleExhausted(context.Background(), "g", "topic", []byte("msg"), errors.New("err"),
		dl, nil, nil, nil)
	assert.Equal(t, exhaustedHandled, result)
	assert.True(t, dl.called.Load())
}

func TestHandleExhausted_WithDeadLetter_Fail(t *testing.T) {
	dl := &failingDeadLetterHandler{}
	var buf bytes.Buffer
	logger := logutil.NewSlogLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	result := handleExhausted(context.Background(), "g", "topic", []byte("msg"), errors.New("err"),
		dl, nil, logger, nil)
	assert.Equal(t, exhaustedFailed, result)
}

func TestHandleExhausted_WithFailedHandler(t *testing.T) {
	fhCalled := false
	fh := types.FailedHandlerFunc(func(ctx context.Context, msg types.Message, err error) {
		fhCalled = true
	})
	result := handleExhausted(context.Background(), "g", "topic", []byte("msg"), errors.New("err"),
		nil, fh, nil, nil)
	assert.Equal(t, exhaustedHandled, result)
	assert.True(t, fhCalled)
}

func TestHandleExhausted_WithDefaultFailedHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := logutil.NewSlogLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	fh := DefaultFailedHandlerFunc(logger)
	result := handleExhausted(context.Background(), "g", "topic", []byte("msg"), errors.New("err"),
		nil, fh, logger, nil)
	assert.Equal(t, exhaustedHandled, result)
	assert.Contains(t, buf.String(), "message consume failed")
}

func TestHandleExhausted_WithMetrics(t *testing.T) {
	m := metrics.NewConsumerMetrics("kafka")
	result := handleExhausted(context.Background(), "g", "topic", []byte("msg"), errors.New("err"),
		nil, nil, nil, m)
	assert.Equal(t, exhaustedHandled, result)
	// Metrics OnDeadLetter should have been called
}

func TestHandleExhausted_NoDeadLetterNoFailedHandlerNoLogger(t *testing.T) {
	result := handleExhausted(context.Background(), "g", "topic", []byte("msg"), errors.New("err"),
		nil, nil, nil, nil)
	assert.Equal(t, exhaustedHandled, result)
}