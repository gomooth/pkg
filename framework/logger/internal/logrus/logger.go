package logrus

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomooth/pkg/framework/logger/internal/types"

	rotate "github.com/lestrrat-go/file-rotatelogs"

	"github.com/save95/xlog"

	ologrus "github.com/sirupsen/logrus"
)

type logger struct {
	fields xlog.Fields

	engine *ologrus.Logger
}

func New(opts ...func(*types.Option)) xlog.XLogger {
	return NewWith(types.GetDefaultDir(), opts...)
}

func NewWith(logPath string, opts ...func(*types.Option)) xlog.XLogger {
	l := &logger{}

	// 创建目录
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		_ = os.MkdirAll(logPath, os.ModePerm)
	}

	cnf := types.DefaultOption()
	for _, opt := range opts {
		opt(cnf)
	}

	// 初始化引擎
	eg := ologrus.New()
	eg.SetFormatter(l.getFormatter(cnf.Format))
	eg.SetLevel(l.getLevel(cnf.Level))
	eg.SetOutput(os.Stdout)

	filename := filepath.Join(logPath, l.getFilenamePatten(cnf.Stack))
	// 打开文件
	rl, err := rotate.New(filename)
	if nil == err {
		eg.SetOutput(rl)
	} else {
		if fp, err := os.Open(filename); err == nil {
			eg.SetOutput(fp)
		}
	}

	if cnf.StdPrint {
		eg.AddHook(newStdoutHook(eg.Formatter))
	}

	l.engine = eg

	return l
}

func (l *logger) getFormatter(format xlog.LogFormat) ologrus.Formatter {
	switch format {
	case xlog.LogFormatJson:
		return &formatJson{}
	default:
		return &formatText{}
	}
}

func (l *logger) getLevel(level xlog.Level) ologrus.Level {
	return ologrus.Level(level)
}

func (l *logger) getFilenamePatten(stack xlog.Stack) string {
	filename := types.GetDefaultFilenameFormat()

	switch stack {
	case xlog.DailyStack:
		filename = "%Y-%m-%d.log"
	case xlog.SingleStack:
		filename = types.GetDefaultFilenameFormat()
	default:
		filename = types.GetDefaultFilenameFormat()
	}

	return filename
}

func (l *logger) prefix(ctx context.Context) string {
	args := make([]string, 0)
	traceText := types.ParseTrace(ctx)
	if len(traceText) > 0 {
		args = append(args, traceText)
	}
	fields := types.ParseField(l.fields)
	if len(fields) > 0 {
		args = append(args, fields)
	}

	prefix := strings.Join(args, " ")
	if len(prefix) > 0 {
		prefix += "\t"
	}
	return prefix
}

func (l *logger) Debug(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	l.engine.Debug(args...)
}

func (l *logger) Info(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	l.engine.Info(args...)
}

func (l *logger) Warning(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	l.engine.Warning(args...)
}

func (l *logger) Error(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	l.engine.Error(args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	l.engine.Debugf(format, args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	l.engine.Infof(format, args...)
}

func (l *logger) Warningf(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	l.engine.Warningf(format, args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	l.engine.Errorf(format, args...)
}

func (l *logger) Debugc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	l.engine.Debugf(format, args...)
}

func (l *logger) Infoc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	l.engine.Infof(format, args...)
}

func (l *logger) Warningc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	l.engine.Warningf(format, args...)
}

func (l *logger) Errorc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	l.engine.Errorf(format, args...)
}

func (l *logger) WithField(key string, value interface{}, options ...interface{}) xlog.XLog {
	nl := *l
	if nl.fields == nil {
		nl.fields = make(xlog.Fields)
	}
	nl.fields[key] = value
	return &nl
}

func (l *logger) WithFields(fields xlog.Fields, options ...interface{}) xlog.XLog {
	nl := *l
	if nl.fields == nil {
		nl.fields = make(xlog.Fields)
	}
	for k, v := range fields {
		nl.fields[k] = v
	}
	return &nl
}
