package types

import "log/slog"

// Level 日志等级，基于 slog.Level
type Level = slog.Level

const (
	DebugLevel = slog.LevelDebug
	InfoLevel  = slog.LevelInfo
	WarnLevel  = slog.LevelWarn
	ErrorLevel = slog.LevelError
)

// ParseLevel 将字符串解析为日志等级
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

// Stack 日志存储方式
type Stack int

const (
	SingleStack Stack = iota
	DailyStack
)

// LogFormat 日志格式
type LogFormat int

const (
	LogFormatText LogFormat = iota
	LogFormatJson
)
