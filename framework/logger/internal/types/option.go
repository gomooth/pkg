package types

import (
	"github.com/save95/xlog"
)

type Option struct {
	Stack  xlog.Stack     // 日志存储方式
	Level  xlog.Level     // 日志等级
	Format xlog.LogFormat // 日志格式

	StdPrint bool // 是否在控制台输出
}

func DefaultOption() *Option {
	return &Option{
		Stack:  xlog.DailyStack,
		Level:  xlog.InfoLevel,
		Format: xlog.LogFormatText,
	}
}
