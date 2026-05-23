package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gomooth/pkg/framework/logger/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewConsoleLogger(t *testing.T) {
	l := NewConsoleLogger()
	assert.NotNil(t, l)
}

func TestNewFileLogger(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewFileLogger(tmpDir)
	assert.NotNil(t, l)

	// 写入日志，验证文件生成
	l.Info("test file logger message")

	// 检查目录下有日志文件
	entries, err := os.ReadDir(tmpDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, entries, "log directory should contain files")
}

func TestNewFileLogger_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewFileLogger(tmpDir,
		WithLevel(types.DebugLevel),
		WithFormat(types.LogFormatJson),
		WithStack(types.DailyStack),
		WithStdPrint(false),
	)
	assert.NotNil(t, l)
	l.Debug("debug message should be visible")
}

func TestSetDefault(t *testing.T) {
	original := slog.Default()

	l := NewConsoleLogger()
	SetDefault(l)
	assert.Equal(t, l, slog.Default())

	// 恢复原始默认值
	slog.SetDefault(original)
}

func TestSetDefault_NilNoPanic(t *testing.T) {
	original := slog.Default()

	// nil 不应 panic，也不应改变默认值
	SetDefault(nil)
	assert.Equal(t, original, slog.Default())
}

func TestWithLevelString(t *testing.T) {
	tests := []struct {
		input    string
		expected types.Level
	}{
		{"debug", types.DebugLevel},
		{"info", types.InfoLevel},
		{"warn", types.WarnLevel},
		{"warning", types.WarnLevel},
		{"error", types.ErrorLevel},
		{"", types.InfoLevel},
		{"unknown", types.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			opt := types.DefaultOption()
			WithLevelString(tt.input)(opt)
			assert.Equal(t, tt.expected, opt.Level)
		})
	}
}

func TestNewFileLogger_DailyStack(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewFileLogger(tmpDir, WithStack(types.DailyStack))
	assert.NotNil(t, l)
	l.Info("daily stack log")

	// 验证日志文件存在
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.log"))
	assert.NotEmpty(t, matches, "should create daily log file")
}

func TestNewConsoleLogger_WithSampling(t *testing.T) {
	l := NewConsoleLogger(
		WithLevel(types.DebugLevel),
		WithSampling(types.SamplingConfig{
			LevelRates: map[slog.Level]float64{
				slog.LevelDebug: 5.0,
			},
		}),
	)
	assert.NotNil(t, l)
	// 不 panic 即可
	l.Debug("sampling test message")
}

func TestNewFileLogger_WithSampling(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewFileLogger(tmpDir,
		WithLevel(types.InfoLevel),
		WithFormat(types.LogFormatJson),
		WithSampling(types.SamplingConfig{
			LevelRates: map[slog.Level]float64{
				slog.LevelInfo: 10.0,
			},
			BurstMultiplier: 2.0,
		}),
	)
	assert.NotNil(t, l)
	l.Info("sampling file log test")
}
