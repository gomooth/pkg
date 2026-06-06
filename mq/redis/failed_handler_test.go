package redis

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/stretchr/testify/assert"
)

func TestDefaultFailedHandlerFunc_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))

	handler(context.Background(), "test-queue", []byte("hello"), errors.New("handle failed"))

	output := buf.String()
	assert.Contains(t, output, "message consume failed")
	assert.Contains(t, output, "redis-consumer")
	assert.Contains(t, output, "test-queue")
	assert.Contains(t, output, "handle failed")
}

func TestDefaultFailedHandlerFunc_WithContextError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler(ctx, "test-queue", []byte("hello"), errors.New("handle failed"))

	output := buf.String()
	assert.Contains(t, output, "context canceled")
}

func TestDefaultFailedHandlerFunc_NilLogger(t *testing.T) {
	handler := DefaultFailedHandlerFunc(nil)
	// 不应 panic
	handler(context.Background(), "q", []byte("m"), errors.New("e"))
}

func TestFailedHandlerFunc(t *testing.T) {
	var called bool
	fn := FailedHandlerFunc(func(ctx context.Context, queue string, message []byte, err error) {
		called = true
	})

	fn(context.Background(), "q", []byte("m"), errors.New("e"))
	assert.True(t, called)
}
