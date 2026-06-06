package retry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/mq/internal/types"
)

type mockDeadLetterHandler struct {
	called bool
	msg    types.Message
	err    error
}

func (m *mockDeadLetterHandler) OnDeadLetter(_ context.Context, msg types.Message, lastErr error) error {
	m.called = true
	m.msg = msg
	m.err = lastErr
	return nil
}

func TestHandleExhausted_WithDeadLetterHandler(t *testing.T) {
	dl := &mockDeadLetterHandler{}
	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	msg := types.NewRedisMessage("q1", []byte("hello"))
	lastErr := errors.New("boom")

	HandleExhausted(context.Background(), nil, dl, failedFn, msg, lastErr)

	assert.True(t, dl.called, "DeadLetterHandler should be called")
	assert.Equal(t, msg, dl.msg)
	assert.Equal(t, lastErr, dl.err)
	assert.False(t, failedCalled, "FailedHandlerFunc should NOT be called when DeadLetterHandler is set")
}

func TestHandleExhausted_WithOnlyFailedHandler(t *testing.T) {
	var failedCalled bool
	var failedMsg types.Message
	var failedErr error
	failedFn := func(_ context.Context, msg types.Message, err error) {
		failedCalled = true
		failedMsg = msg
		failedErr = err
	}

	msg := types.NewKafkaMessage("g1", "topic1", []byte("data"))
	lastErr := errors.New("fail")

	HandleExhausted(context.Background(), nil, nil, failedFn, msg, lastErr)

	assert.True(t, failedCalled, "FailedHandlerFunc should be called")
	assert.Equal(t, msg, failedMsg)
	assert.Equal(t, lastErr, failedErr)
}

func TestHandleExhausted_DeadLetterTakesPrecedence(t *testing.T) {
	dl := &mockDeadLetterHandler{}
	var failedCalled bool
	failedFn := func(_ context.Context, _ types.Message, _ error) {
		failedCalled = true
	}

	HandleExhausted(context.Background(), nil, dl, failedFn, types.NewRedisMessage("q", nil), errors.New("e"))

	assert.True(t, dl.called, "DeadLetterHandler should be called")
	assert.False(t, failedCalled, "FailedHandlerFunc should NOT be called")
}

func TestHandleExhausted_NeitherSet(t *testing.T) {
	// Should not panic
	HandleExhausted(context.Background(), nil, nil, nil, types.NewRedisMessage("q", nil), errors.New("e"))
}
