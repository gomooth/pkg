package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	intotel "github.com/gomooth/pkg/framework/telemetry/internal/otel"
)

// Provider 可观测性统一提供者，聚合 Tracing/Metrics/Logs 三大信号。
// 返回 OTel 原生接口，不做委托包装。
type Provider interface {
	// TracerProvider 返回链路追踪提供者
	TracerProvider() trace.TracerProvider

	// MeterProvider 返回指标提供者
	MeterProvider() metric.MeterProvider

	// LoggerProvider 返回日志提供者（可选，可能为 nil）
	LoggerProvider() log.LoggerProvider

	// Shutdown 优雅关闭所有 Provider
	Shutdown(ctx context.Context) error
}

// otelProvider 适配 internal otel provider，实现 Provider 接口
type otelProvider struct {
	inner *intotel.Provider
}

// NewProvider 创建 OTel Provider
func NewProvider(opts ...Option) Provider {
	return &otelProvider{inner: intotel.New(opts...)}
}

func (p *otelProvider) TracerProvider() trace.TracerProvider {
	return p.inner.TracerProvider()
}

func (p *otelProvider) MeterProvider() metric.MeterProvider {
	return p.inner.MeterProvider()
}

func (p *otelProvider) LoggerProvider() log.LoggerProvider {
	return p.inner.LoggerProvider()
}

func (p *otelProvider) Shutdown(ctx context.Context) error {
	return p.inner.Shutdown(ctx)
}

// 全局 Provider，默认空操作（零开销）
var globalProvider Provider = &noopProvider{}

// SetProvider 设置全局可观测性 Provider。
// 同时自动同步 OTel 全局 TracerProvider 和 MeterProvider，确保第三方库也能受益。
// 传入 nil 重置为 noopProvider。
func SetProvider(p Provider) {
	if p != nil {
		globalProvider = p
	} else {
		globalProvider = &noopProvider{}
	}

	// 同步 OTel 全局
	if tp := globalProvider.TracerProvider(); tp != nil {
		otel.SetTracerProvider(tp)
	}
	if mp := globalProvider.MeterProvider(); mp != nil {
		otel.SetMeterProvider(mp)
	}
}

// GetProvider 获取全局可观测性 Provider
func GetProvider() Provider {
	return globalProvider
}

// Tracer 快捷方式，返回 OTel 原生 trace.Tracer
func Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return GetProvider().TracerProvider().Tracer(name, opts...)
}

// Meter 快捷方式，返回 OTel 原生 metric.Meter
func Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return GetProvider().MeterProvider().Meter(name, opts...)
}

// noopProvider 未配置时的空操作实现
type noopProvider struct{}

func (p *noopProvider) TracerProvider() trace.TracerProvider {
	return tracenoop.NewTracerProvider()
}

func (p *noopProvider) MeterProvider() metric.MeterProvider {
	return metricnoop.NewMeterProvider()
}

func (p *noopProvider) LoggerProvider() log.LoggerProvider { return nil }

func (p *noopProvider) Shutdown(_ context.Context) error { return nil }
