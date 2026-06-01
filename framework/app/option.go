package app

import (
	"log/slog"
	"time"
)

// WithLogger 设置应用管理器的日志器，默认使用控制台日志
func WithLogger(log *slog.Logger) func(*manager) {
	return func(m *manager) {
		m.log = log
	}
}

// WithApp 注册一个应用实例到管理器
func WithApp(app IApp) func(*manager) {
	return func(m *manager) {
		if m.apps == nil {
			m.apps = make([]IApp, 0)
		}
		m.Register(app)
	}
}

// WithShutdownTimeout 设置优雅关闭超时时间，默认 30 秒
func WithShutdownTimeout(d time.Duration) func(*manager) {
	return func(m *manager) {
		if d > 0 {
			m.shutdownTimeout = d
		}
	}
}

// WithStartupTimeout 设置单个应用启动超时时间，0 表示不限制
func WithStartupTimeout(d time.Duration) func(*manager) {
	return func(m *manager) {
		m.startupTimeout = d
	}
}
