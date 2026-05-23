package logger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-resty/resty/v2"
)

type apiLogger struct {
	l *slog.Logger
}

// RestyWrapper Resty 日志包装器
func RestyWrapper(l *slog.Logger) resty.Logger {
	return &apiLogger{l: l}
}

func (a *apiLogger) Errorf(format string, v ...interface{}) {
	a.l.LogAttrs(context.Background(), slog.LevelError, fmt.Sprintf(format, v...))
}

func (a *apiLogger) Warnf(format string, v ...interface{}) {
	a.l.LogAttrs(context.Background(), slog.LevelWarn, fmt.Sprintf(format, v...))
}

func (a *apiLogger) Debugf(format string, v ...interface{}) {
	a.l.LogAttrs(context.Background(), slog.LevelDebug, fmt.Sprintf(format, v...))
}
