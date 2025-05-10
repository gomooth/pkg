package console

import (
	"context"
	"log"
	"strings"

	"github.com/gomooth/pkg/framework/logger/internal/types"

	"github.com/save95/xlog"
)

type logger struct {
	fields xlog.Fields
}

func New() xlog.XLogger {
	l := &logger{}

	return l
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
	log.Print(args...)
}

func (l *logger) Info(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	log.Print(args...)
}

func (l *logger) Warning(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	log.Print(args...)
}

func (l *logger) Error(args ...interface{}) {
	args = append([]interface{}{l.prefix(nil)}, args...)
	log.Print(args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	log.Printf(format, args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	log.Printf(format, args...)
}

func (l *logger) Warningf(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	log.Printf(format, args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	format = l.prefix(nil) + format
	log.Printf(format, args...)
}

func (l *logger) Debugc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	log.Printf(format, args...)
}

func (l *logger) Infoc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	log.Printf(format, args...)
}

func (l *logger) Warningc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	log.Printf(format, args...)
}

func (l *logger) Errorc(ctx context.Context, format string, args ...interface{}) {
	format = l.prefix(ctx) + format
	log.Printf(format, args...)
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
