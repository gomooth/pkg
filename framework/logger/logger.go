package logger

import (
	"github.com/gomooth/pkg/framework/logger/internal/console"
	"github.com/gomooth/pkg/framework/logger/internal/logrus"
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
