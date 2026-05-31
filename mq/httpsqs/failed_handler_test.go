package httpsqs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultFailedHandler_Print(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := newDefaultFailedHandler(logger)

	handler.Print(context.Background(), "test-queue", "hello", 42, errors.New("handle failed"))

	output := buf.String()
	assert.Contains(t, output, "message consume failed")
	assert.Contains(t, output, "httpsqs-consumer")
	assert.Contains(t, output, "test-queue")
	assert.Contains(t, output, "handle failed")
}

func TestDefaultFailedHandler_Print_WithContextError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := newDefaultFailedHandler(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler.Print(ctx, "test-queue", "hello", 42, errors.New("handle failed"))

	output := buf.String()
	assert.Contains(t, output, "context canceled")
}

func TestDefaultFailedHandler_NilLogger(t *testing.T) {
	handler := newDefaultFailedHandler(nil)
	assert.NotNil(t, handler)
	// 不应 panic
	handler.Print(context.Background(), "q", "m", 1, errors.New("e"))
}
