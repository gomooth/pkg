package filelog

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/gomooth/pkg/framework/logger/internal/multihandler"
	"github.com/gomooth/pkg/framework/logger/internal/sampling"
	"github.com/gomooth/pkg/framework/logger/internal/trace"
	"github.com/gomooth/pkg/framework/logger/internal/types"
	"gopkg.in/natefinch/lumberjack.v2"
)

// New 创建文件日志器，使用默认日志目录
func New(opts ...func(*types.Option)) *slog.Logger {
	return NewWith(types.GetDefaultDir(), opts...)
}

// NewWith 创建文件日志器，支持日志文件自动分割，可选配置 OTel LoggerProvider 启用 OTLP 日志导出。
func NewWith(logPath string, opts ...func(*types.Option)) *slog.Logger {
	// 创建目录
	if _, err := os.Stat(logPath); errors.Is(err, fs.ErrNotExist) {
		_ = os.MkdirAll(logPath, os.ModePerm)
	}

	cnf := types.DefaultOption()
	for _, opt := range opts {
		opt(cnf)
	}

	// 构建输出
	var output io.Writer = os.Stdout

	filename := filepath.Join(logPath, getFilename(cnf.Stack))
	lj := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100, // MB，单文件最大尺寸
		MaxAge:     30,  // 天，保留最近30天的日志
		MaxBackups: 0,   // 0 表示不限制备份数量，由 MaxAge 控制
		LocalTime:  true,
		Compress:   true,
	}
	output = lj

	// StdPrint: 同时输出到文件和控制台
	if cnf.StdPrint {
		output = io.MultiWriter(output, os.Stdout)
	}

	// 构建 handler
	handlerOpts := &slog.HandlerOptions{
		Level: cnf.Level,
	}

	var handler slog.Handler
	switch cnf.Format {
	case types.LogFormatJson:
		handler = slog.NewJSONHandler(output, handlerOpts)
	default:
		handler = slog.NewTextHandler(output, handlerOpts)
	}

	// 采样（在 trace.Injector 之前，避免对将被丢弃的日志注入 trace）
	if cnf.Sampling != nil && len(cnf.Sampling.LevelRates) > 0 {
		handler = sampling.New(handler, sampling.Config{
			LevelRates:      cnf.Sampling.LevelRates,
			BurstMultiplier: cnf.Sampling.BurstMultiplier,
			SummaryInterval: cnf.Sampling.SummaryInterval,
		})
	}

	// 注入 trace_id / span_id
	handler = &trace.Injector{Next: handler}

	// OTLP 日志导出（opt-in）
	if cnf.OTelLoggerProvider != nil {
		otelHandler := otelslog.NewHandler("pkg/filelog", otelslog.WithLoggerProvider(cnf.OTelLoggerProvider))
		handler = multihandler.New(handler, otelHandler)
	}

	return slog.New(handler)
}

func getFilename(stack types.Stack) string {
	switch stack {
	case types.DailyStack:
		return time.Now().Format("2006-01-02") + ".log"
	default:
		return types.GetDefaultFilenameFormat()
	}
}
