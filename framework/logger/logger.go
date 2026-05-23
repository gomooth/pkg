package logger

import (
	"log/slog"

	"github.com/gomooth/pkg/framework/logger/internal/console"
	"github.com/gomooth/pkg/framework/logger/internal/filelog"
	"github.com/gomooth/pkg/framework/logger/internal/types"
)

// NewFileLogger 创建文件日志器，支持日志文件自动分割
func NewFileLogger(path string, opts ...func(*types.Option)) *slog.Logger {
	return filelog.NewWith(path, opts...)
}

// NewConsoleLogger 创建控制台日志器
func NewConsoleLogger(opts ...func(*types.Option)) *slog.Logger {
	return console.New(opts...)
}

// SetDefault 设置 slog 默认日志器，设置后可直接使用 slog.Info() 等全局方法
func SetDefault(l *slog.Logger) {
	if l != nil {
		slog.SetDefault(l)
	}
}
