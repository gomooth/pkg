package internal

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
)

func TestInitSaramaLogger_SetsGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	InitSaramaLogger(logger)

	// 通过 sarama.Logger 接口写入
	sarama.Logger.Println("test-message")

	if !strings.Contains(buf.String(), "test-message") {
		t.Errorf("expected sarama log to contain 'test-message', got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "component=kafka") {
		t.Errorf("expected sarama log to contain 'component=kafka', got: %s", buf.String())
	}
}

func TestSaramaLogAdapter_Print(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := &saramaLogAdapter{l: logger}

	assert.NotPanics(t, func() {
		adapter.Print("hello", "world")
	}, "Print should not panic")
	assert.Contains(t, buf.String(), "hello", "Print output should contain message")
}

func TestSaramaLogAdapter_Printf(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := &saramaLogAdapter{l: logger}

	assert.NotPanics(t, func() {
		adapter.Printf("value=%d", 42)
	}, "Printf should not panic")
	assert.Contains(t, buf.String(), "value=42", "Printf output should contain formatted message")
}

func TestSaramaLogAdapter_Println(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := &saramaLogAdapter{l: logger}

	assert.NotPanics(t, func() {
		adapter.Println("line1", "line2")
	}, "Println should not panic")
	assert.Contains(t, buf.String(), "line1", "Println output should contain message")
}
