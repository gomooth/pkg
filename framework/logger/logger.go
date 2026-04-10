package logger

import (
	"github.com/gomooth/pkg/framework/logger/internal/console"
	"github.com/gomooth/pkg/framework/logger/internal/logrus"
	"github.com/gomooth/pkg/framework/logger/internal/slog"
	"github.com/gomooth/pkg/framework/logger/internal/types"

	"github.com/save95/xlog"
)

// NewLogrusLogger 文件日志器。使用 logrus 实现，支持日志文件自动分割
func NewLogrusLogger(path string, opts ...func(*types.Option)) xlog.XLogger {
	return logrus.NewWith(path, opts...)
}

// NewConsoleLogger 控制台日志器
func NewConsoleLogger() xlog.XLogger {
	return console.New()
}

// NewSlogLogger 文件日志器。使用 slog 实现，支持日志文件自动分割
func NewSlogLogger(path string, opts ...func(*types.Option)) xlog.XLogger {
	return slog.NewWith(path, opts...)
}

// InitSlog 初始化 slog 的默认 logger，使业务侧可以直接使用 slog 包
func InitSlog(logger xlog.XLogger) {
	slog.Init(logger)
}
