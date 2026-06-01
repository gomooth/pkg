package telemetry

import (
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	intotel "github.com/gomooth/pkg/framework/telemetry/internal/otel"
)

// Option telemetry Provider 配置项
type Option = intotel.Option

// WithTracerProvider 设置链路追踪提供者
func WithTracerProvider(tp trace.TracerProvider) Option {
	return intotel.WithTracerProvider(tp)
}

// WithMeterProvider 设置指标提供者
func WithMeterProvider(mp metric.MeterProvider) Option {
	return intotel.WithMeterProvider(mp)
}

// WithLoggerProvider 设置日志提供者
func WithLoggerProvider(lp log.LoggerProvider) Option {
	return intotel.WithLoggerProvider(lp)
}
