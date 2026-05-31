package internal

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/IBM/sarama"
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
