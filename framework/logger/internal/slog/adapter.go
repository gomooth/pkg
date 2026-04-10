package slog

import (
	"context"
	"log/slog"

	"github.com/save95/xlog"
)

// Init 初始化 slog 的默认 logger，使业务侧可以直接使用 slog 包
func Init(logger xlog.XLogger) {
	if logger == nil {
		return
	}

	// 创建一个适配器，将 slog 的调用转发到传入的 logger
	handler := &slogAdapter{logger: logger}
	slog.SetDefault(slog.New(handler))
}

// slogAdapter 实现 slog.Handler 接口，将 slog 调用适配到 xlog.XLogger
type slogAdapter struct {
	logger xlog.XLogger
	attrs  []slog.Attr
	groups []string
}

func (a *slogAdapter) Enabled(ctx context.Context, level slog.Level) bool {
	// 简单的级别转换
	xlogLevel := slogLevelToXLog(level)
	return isLevelEnabled(a.logger, xlogLevel)
}

func (a *slogAdapter) Handle(ctx context.Context, r slog.Record) error {
	// 构建消息
	msg := r.Message

	// 收集所有属性
	attrs := make([]slog.Attr, 0, len(a.attrs)+r.NumAttrs())
	attrs = append(attrs, a.attrs...)
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	// 将属性转换为 fields
	fields := make(xlog.Fields)
	for _, attr := range attrs {
		key := attr.Key
		// 处理分组
		for _, group := range a.groups {
			key = group + "." + key
		}
		fields[key] = attr.Value.Any()
	}

	// 根据级别记录日志
	var logFunc func(context.Context, string, ...interface{})

	switch r.Level {
	case slog.LevelDebug:
		logFunc = a.logger.Debugc
	case slog.LevelInfo:
		logFunc = a.logger.Infoc
	case slog.LevelWarn:
		logFunc = a.logger.Warningc
	case slog.LevelError:
		logFunc = a.logger.Errorc
	default:
		return nil
	}

	// 如果有字段，使用 WithFields
	if len(fields) > 0 {
		loggerWithFields := a.logger.WithFields(fields)
		switch r.Level {
		case slog.LevelDebug:
			loggerWithFields.Debugc(ctx, msg)
		case slog.LevelInfo:
			loggerWithFields.Infoc(ctx, msg)
		case slog.LevelWarn:
			loggerWithFields.Warningc(ctx, msg)
		case slog.LevelError:
			loggerWithFields.Errorc(ctx, msg)
		}
	} else {
		logFunc(ctx, msg)
	}

	return nil
}

func (a *slogAdapter) WithAttrs(attrs []slog.Attr) slog.Handler {
	// 复制适配器并添加属性
	na := *a
	na.attrs = make([]slog.Attr, len(a.attrs)+len(attrs))
	copy(na.attrs, a.attrs)
	copy(na.attrs[len(a.attrs):], attrs)
	return &na
}

func (a *slogAdapter) WithGroup(name string) slog.Handler {
	// 复制适配器并添加分组
	na := *a
	na.groups = make([]string, len(a.groups)+1)
	copy(na.groups, a.groups)
	na.groups[len(na.groups)-1] = name
	return &na
}

// slogLevelToXLog 将 slog.Level 转换为 xlog.Level
func slogLevelToXLog(level slog.Level) xlog.Level {
	switch {
	case level <= slog.LevelDebug:
		return xlog.DebugLevel
	case level <= slog.LevelInfo:
		return xlog.InfoLevel
	case level <= slog.LevelWarn:
		return xlog.WarnLevel
	default:
		return xlog.ErrorLevel
	}
}

// isLevelEnabled 检查指定级别是否启用
func isLevelEnabled(logger xlog.XLogger, level xlog.Level) bool {
	// 这是一个简化实现，实际应该根据 logger 的配置检查
	// 这里假设所有级别都启用
	return true
}
