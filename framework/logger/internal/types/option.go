package types

import (
	"log/slog"
	"time"
)

type Option struct {
	Stack  Stack     // 日志存储方式
	Level  Level     // 日志等级
	Format LogFormat // 日志格式

	StdPrint bool // 是否在控制台输出

	// Sampling 采样配置，nil 表示不启用采样
	Sampling *SamplingConfig
}

// SamplingConfig 日志采样配置
type SamplingConfig struct {
	// LevelRates 按 slog.Level 配置每秒允许的日志条数。
	// 未配置的级别不限流，0 表示该级别不限。
	LevelRates map[slog.Level]float64

	// BurstMultiplier 突发容量倍数，burst = max(1, int(rate * BurstMultiplier))，默认 1.0
	BurstMultiplier float64

	// SummaryInterval 丢弃摘要间隔，0 表示不输出摘要
	SummaryInterval time.Duration
}

func DefaultOption() *Option {
	return &Option{
		Stack:  DailyStack,
		Level:  slog.LevelInfo,
		Format: LogFormatText,
	}
}
