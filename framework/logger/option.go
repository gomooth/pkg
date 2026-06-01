package logger

import (
	"github.com/gomooth/pkg/framework/logger/internal/types"
)

// WithStack 设置日志存储方式
func WithStack(stack types.Stack) func(*types.Option) {
	return func(o *types.Option) {
		o.Stack = stack
	}
}

// WithLevel 设置日志等级
func WithLevel(level types.Level) func(*types.Option) {
	return func(o *types.Option) {
		o.Level = level
	}
}

// WithLevelString 通过日志等级字符串设置
// 支持日志等级：debug, info, warn, error
func WithLevelString(levelText string) func(*types.Option) {
	return func(o *types.Option) {
		if len(levelText) > 0 {
			o.Level = types.ParseLevel(levelText)
		}
	}
}

// WithFormat 设置日志格式
func WithFormat(format types.LogFormat) func(*types.Option) {
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

// WithSampling 设置日志采样配置。
// 启用后，超出速率限制的日志将被丢弃，可选输出丢弃摘要。
// 未配置的级别不限流，Error 级别默认不限（除非在 LevelRates 中显式配置）。
func WithSampling(cfg types.SamplingConfig) func(*types.Option) {
	return func(o *types.Option) {
		o.Sampling = &cfg
	}
}
