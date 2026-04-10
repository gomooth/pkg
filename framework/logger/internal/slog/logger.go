package slog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomooth/pkg/framework/logger/internal/types"

	rotate "github.com/lestrrat-go/file-rotatelogs"

	"github.com/save95/xlog"
)

type logger struct {
	fields xlog.Fields
	engine *slog.Logger
	level  xlog.Level
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
	var handler slog.Handler
	var output io.Writer = os.Stdout

	filename := filepath.Join(logPath, getFilenamePatten(cnf.Stack))
	// 打开文件
	rl, err := rotate.New(filename)
	if nil == err {
		output = rl
	} else {
		if fp, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			output = fp
		}
	}

	switch cnf.Format {
	case xlog.LogFormatJson:
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level: getSlogLevel(cnf.Level),
		})
	default:
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level: getSlogLevel(cnf.Level),
		})
	}

	l.engine = slog.New(handler)
	l.level = cnf.Level

	return l
}

func getSlogLevel(level xlog.Level) slog.Level {
	switch level {
	case xlog.DebugLevel:
		return slog.LevelDebug
	case xlog.InfoLevel:
		return slog.LevelInfo
	case xlog.WarnLevel:
		return slog.LevelWarn
	case xlog.ErrorLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getFilenamePatten(stack xlog.Stack) string {
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

func (l *logger) prefix(ctx context.Context) []slog.Attr {
	attrs := make([]slog.Attr, 0)

	// 添加 trace 信息
	traceText := types.ParseTrace(ctx)
	if len(traceText) > 0 {
		attrs = append(attrs, slog.String("trace", traceText))
	}

	// 添加 fields
	if l.fields != nil {
		for k, v := range l.fields {
			attrs = append(attrs, slog.Any(k, v))
		}
	}

	return attrs
}

func (l *logger) log(ctx context.Context, level slog.Level, msg string, args ...interface{}) {
	attrs := l.prefix(ctx)
	if len(args) > 0 {
		// 如果有参数，将其作为附加属性
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				attrs = append(attrs, slog.Any(args[i].(string), args[i+1]))
			}
		}
	}
	l.engine.LogAttrs(ctx, level, msg, attrs...)
}

func (l *logger) logf(ctx context.Context, level slog.Level, format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = strings.TrimSpace(fmt.Sprintf(format, args...))
	}
	l.log(ctx, level, msg)
}

func (l *logger) Debug(args ...interface{}) {
	if l.level > xlog.DebugLevel {
		return
	}
	msg := joinArgs(args...)
	l.log(context.Background(), slog.LevelDebug, msg)
}

func (l *logger) Info(args ...interface{}) {
	if l.level > xlog.InfoLevel {
		return
	}
	msg := joinArgs(args...)
	l.log(context.Background(), slog.LevelInfo, msg)
}

func (l *logger) Warning(args ...interface{}) {
	if l.level > xlog.WarnLevel {
		return
	}
	msg := joinArgs(args...)
	l.log(context.Background(), slog.LevelWarn, msg)
}

func (l *logger) Error(args ...interface{}) {
	if l.level > xlog.ErrorLevel {
		return
	}
	msg := joinArgs(args...)
	l.log(context.Background(), slog.LevelError, msg)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	if l.level > xlog.DebugLevel {
		return
	}
	l.logf(context.Background(), slog.LevelDebug, format, args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	if l.level > xlog.InfoLevel {
		return
	}
	l.logf(context.Background(), slog.LevelInfo, format, args...)
}

func (l *logger) Warningf(format string, args ...interface{}) {
	if l.level > xlog.WarnLevel {
		return
	}
	l.logf(context.Background(), slog.LevelWarn, format, args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	if l.level > xlog.ErrorLevel {
		return
	}
	l.logf(context.Background(), slog.LevelError, format, args...)
}

func (l *logger) Debugc(ctx context.Context, format string, args ...interface{}) {
	if l.level > xlog.DebugLevel {
		return
	}
	l.logf(ctx, slog.LevelDebug, format, args...)
}

func (l *logger) Infoc(ctx context.Context, format string, args ...interface{}) {
	if l.level > xlog.InfoLevel {
		return
	}
	l.logf(ctx, slog.LevelInfo, format, args...)
}

func (l *logger) Warningc(ctx context.Context, format string, args ...interface{}) {
	if l.level > xlog.WarnLevel {
		return
	}
	l.logf(ctx, slog.LevelWarn, format, args...)
}

func (l *logger) Errorc(ctx context.Context, format string, args ...interface{}) {
	if l.level > xlog.ErrorLevel {
		return
	}
	l.logf(ctx, slog.LevelError, format, args...)
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

func joinArgs(args ...interface{}) string {
	if len(args) == 0 {
		return ""
	}
	strs := make([]string, len(args))
	for i, arg := range args {
		strs[i] = fmt.Sprint(arg)
	}
	return strings.Join(strs, " ")
}
