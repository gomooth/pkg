package app

import "github.com/save95/xlog"

func WithLogger(log xlog.XLog) func(*manager) {
	return func(m *manager) {
		m.log = log
	}
}

func WithApp(app IApp) func(*manager) {
	return func(m *manager) {
		if m.apps == nil {
			m.apps = make([]IApp, 0)
		}
		m.Register(app)
	}
}
