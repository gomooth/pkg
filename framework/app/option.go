package app

import (
	"log/slog"
	"time"
)

// WithLogger 设置应用管理器的日志器，默认使用控制台日志
func WithLogger(log *slog.Logger) ManagerOption {
	return func(o *managerOption) {
		o.log = log
	}
}

// WithApp 注册一个应用实例到管理器
func WithApp(app IApp) ManagerOption {
	return func(o *managerOption) {
		o.apps = append(o.apps, app)
	}
}

// WithShutdownTimeout 设置优雅关闭超时时间，默认 30 秒
func WithShutdownTimeout(d time.Duration) ManagerOption {
	return func(o *managerOption) {
		if d > 0 {
			o.shutdownTimeout = d
		}
	}
}

// WithStartupTimeout 设置单个应用启动超时时间，0 表示不限制
func WithStartupTimeout(d time.Duration) ManagerOption {
	return func(o *managerOption) {
		o.startupTimeout = d
	}
}