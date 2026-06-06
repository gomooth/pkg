package logutil

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSlogLogger_NilInput(t *testing.T) {
	logger := NewSlogLogger(nil)
	assert.NotNil(t, logger)
	// 不应 panic
	logger.Debug("test debug")
	logger.Info("test info")
	logger.Warn("test warn")
	logger.Error("test error")
}

func TestNewSlogLogger_WithLogger(t *testing.T) {
	slogger := slog.Default()
	logger := NewSlogLogger(slogger)
	assert.NotNil(t, logger)
}

func TestSlogLogger_AllLevels(t *testing.T) {
	logger := NewSlogLogger(slog.Default())
	assert.NotPanics(t, func() {
		logger.Debug("debug message", "key", "value")
		logger.Info("info message", "key", "value")
		logger.Warn("warn message", "key", "value")
		logger.Error("error message", "key", "value")
	})
}
