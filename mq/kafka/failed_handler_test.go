package kafka

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
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
