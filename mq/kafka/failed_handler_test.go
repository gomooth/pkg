package kafka

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultFailedHandler_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := newDefaultFailedHandler(logger)
	handler.Print(context.Background(), "test-group", "test-topic", []byte("msg"), nil)

	output := buf.String()
	if !strings.Contains(output, "test-group") {
		t.Error("expected log to contain consumer group")
	}
	if !strings.Contains(output, "test-topic") {
		t.Error("expected log to contain topic")
	}
}

func TestDefaultFailedHandler_NilLogger(t *testing.T) {
	handler := newDefaultFailedHandler(nil)
	// 不应 panic
	handler.Print(context.Background(), "group", "topic", []byte("msg"), nil)
}

func TestDefaultFailedHandler_CancelledContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := newDefaultFailedHandler(logger)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler.Print(ctx, "group", "topic", []byte("msg"), nil)
	if !strings.Contains(buf.String(), "context") {
		t.Error("expected log to mention context cancellation")
	}
}

func TestDefaultFailedHandler_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := newDefaultFailedHandler(logger)
	handler.Print(context.Background(), "group", "topic", []byte("msg"), assert.AnError)

	output := buf.String()
	assert.Contains(t, output, "assert") // assert.AnError contains "assert"
	assert.Contains(t, output, "error")
}

func TestDefaultFailedHandler_NilError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := newDefaultFailedHandler(logger)
	handler.Print(context.Background(), "group", "topic", []byte("msg"), nil)

	output := buf.String()
	assert.Contains(t, output, "message consume failed")
}

func TestDefaultFailedHandler_ActiveContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := newDefaultFailedHandler(logger)
	handler.Print(context.Background(), "group", "topic", []byte("msg"), nil)

	output := buf.String()
	// Active context should NOT mention contextErr
	assert.NotContains(t, output, "contextErr")
}
