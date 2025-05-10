package logger

import (
	"github.com/gomooth/pkg/framework/logger/internal/types"

	"github.com/save95/xlog"
)

// WithStack 设置日志存储方式
func WithStack(stack xlog.Stack) func(*types.Option) {
	return func(o *types.Option) {
		o.Stack = stack
	}
}

// WithLevel 设置日志等级
func WithLevel(level xlog.Level) func(*types.Option) {
	return func(o *types.Option) {
		o.Level = level
	}
}

// WithLevelString 通过日志等级字符串设置
// 支持日志等级： panic, fatal, error, warn, info, debug, trace
func WithLevelString(levelText string) func(*types.Option) {
	return func(o *types.Option) {
		if len(levelText) > 0 {
			o.Level = xlog.ParseLevel(levelText)
		}
	}
}

// WithFormat 设置日志格式
func WithFormat(format xlog.LogFormat) func(*types.Option) {
	return func(o *types.Option) {
		o.Format = format
	}
}

// WithStdPrint 设置是否控制台输出
func WithStdPrint(enabled bool) func(*types.Option) {
	return func(o *types.Option) {
		o.StdPrint = enabled
	}
}
