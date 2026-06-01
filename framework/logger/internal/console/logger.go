package console

import (
	"log/slog"
	"os"

	"github.com/gomooth/pkg/framework/logger/internal/sampling"
	"github.com/gomooth/pkg/framework/logger/internal/trace"
	"github.com/gomooth/pkg/framework/logger/internal/types"
)

// New 创建控制台日志器，可选配置 OTel LoggerProvider 启用 OTLP 日志导出。
func New(opts ...func(*types.Option)) *slog.Logger {
	cnf := types.DefaultOption()
	for _, opt := range opts {
		opt(cnf)
	}

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cnf.Level,
	})

	// 采样（在 trace.Injector 之前，避免对将被丢弃的日志注入 trace）
	if cnf.Sampling != nil && len(cnf.Sampling.LevelRates) > 0 {
		handler = sampling.New(handler, sampling.Config{
			LevelRates:      cnf.Sampling.LevelRates,
			BurstMultiplier: cnf.Sampling.BurstMultiplier,
			SummaryInterval: cnf.Sampling.SummaryInterval,
		})
	}

	// 注入 trace_id / span_id
	handler = &trace.Injector{Next: handler}

	return slog.New(handler)
}
