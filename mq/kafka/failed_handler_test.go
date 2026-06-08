package kafka

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestDefaultFailedHandlerFunc_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))
	msg := types.NewKafkaMessage("test-group", "test-topic", []byte("msg"))
	handler(context.Background(), msg, nil)

	output := buf.String()
	if !strings.Contains(output, "test-group") {
		t.Error("expected log to contain consumer group")
	}
	if !strings.Contains(output, "test-topic") {
		t.Error("expected log to contain topic")
	}
}

func TestDefaultFailedHandlerFunc_NilLogger(t *testing.T) {
	handler := DefaultFailedHandlerFunc(nil)
	// 不应 panic
	msg := types.NewKafkaMessage("group", "topic", []byte("msg"))
	handler(context.Background(), msg, nil)
}

func TestDefaultFailedHandlerFunc_CancelledContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := types.NewKafkaMessage("group", "topic", []byte("msg"))
	handler(ctx, msg, nil)
	if !strings.Contains(buf.String(), "context") {
		t.Error("expected log to mention context cancellation")
	}
}

func TestDefaultFailedHandlerFunc_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))
	msg := types.NewKafkaMessage("group", "topic", []byte("msg"))
	handler(context.Background(), msg, assert.AnError)

	output := buf.String()
	assert.Contains(t, output, "assert") // assert.AnError contains "assert"
	assert.Contains(t, output, "error")
}

func TestDefaultFailedHandlerFunc_NilError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))
	msg := types.NewKafkaMessage("group", "topic", []byte("msg"))
	handler(context.Background(), msg, nil)

	output := buf.String()
	assert.Contains(t, output, "message consume failed")
}

func TestDefaultFailedHandlerFunc_ActiveContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))
	msg := types.NewKafkaMessage("group", "topic", []byte("msg"))
	handler(context.Background(), msg, nil)

	output := buf.String()
	// Active context should NOT mention contextErr
	assert.NotContains(t, output, "contextErr")
}