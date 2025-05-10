package logger

import (
	"github.com/go-resty/resty/v2"
	"github.com/save95/xlog"
)

type apiLogger struct {
	xl xlog.XLogger
}

// RestyWrapper Resty 日志包装器
func RestyWrapper(l xlog.XLogger) resty.Logger {
	return &apiLogger{
		xl: l,
	}
}

func (l *apiLogger) Errorf(format string, v ...interface{}) {
	l.xl.Errorf(format, v...)
}

func (l *apiLogger) Warnf(format string, v ...interface{}) {
	l.xl.Warningf(format, v...)
}

func (l *apiLogger) Debugf(format string, v ...interface{}) {
	l.xl.Debugf(format, v...)
}
