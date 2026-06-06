package httpsqs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestDefaultFailedHandlerFunc_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))

	msg := types.NewHttpsqSMessage("test-queue", []byte("hello"), 42)
	handler(context.Background(), msg, errors.New("handle failed"))

	output := buf.String()
	assert.Contains(t, output, "message consume failed")
	assert.Contains(t, output, "httpsqs-consumer")
	assert.Contains(t, output, "test-queue")
	assert.Contains(t, output, "handle failed")
}

func TestDefaultFailedHandlerFunc_WithContextError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := types.NewHttpsqSMessage("test-queue", []byte("hello"), 42)
	handler(ctx, msg, errors.New("handle failed"))

	output := buf.String()
	assert.Contains(t, output, "context canceled")
}

func TestDefaultFailedHandlerFunc_NilLogger(t *testing.T) {
	handler := DefaultFailedHandlerFunc(nil)
	// 不应 panic
	msg := types.NewHttpsqSMessage("q", []byte("m"), 1)
	handler(context.Background(), msg, errors.New("e"))
}