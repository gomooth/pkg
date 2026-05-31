package internal

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSlogLogger(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	logger := NewSlogLogger(l)
	assert.NotNil(t, logger)
}

func TestNewSlogLogger_Nil(t *testing.T) {
	logger := NewSlogLogger(nil)
	assert.NotNil(t, logger)
}

func TestSlogLogger_Methods(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(l)

	logger.Debug("debug msg", "key", "val")
	logger.Info("info msg", "key", "val")
	logger.Warn("warn msg", "key", "val")
	logger.Error("error msg", "key", "val")

	output := buf.String()
	assert.Contains(t, output, "debug msg")
	assert.Contains(t, output, "info msg")
	assert.Contains(t, output, "warn msg")
	assert.Contains(t, output, "error msg")
}
