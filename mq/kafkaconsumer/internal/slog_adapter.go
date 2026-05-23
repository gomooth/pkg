package internal

import "log/slog"

// slogLogger 适配 slog.Logger 到 Logger 接口
type slogLogger struct {
	inner *slog.Logger
}

func newSlogLogger(l *slog.Logger) Logger {
	if l == nil {
		return &slogLogger{inner: slog.Default()}
	}
	return &slogLogger{inner: l}
}

func (s *slogLogger) Debug(msg string, args ...any) { s.inner.Debug(msg, args...) }
func (s *slogLogger) Info(msg string, args ...any)  { s.inner.Info(msg, args...) }
func (s *slogLogger) Warn(msg string, args ...any)  { s.inner.Warn(msg, args...) }
func (s *slogLogger) Error(msg string, args ...any) { s.inner.Error(msg, args...) }
