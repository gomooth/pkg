package otel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Option Provider 配置项
type Option func(*config)

// config Provider 内部配置
type config struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	loggerProvider log.LoggerProvider
}

// WithTracerProvider 设置链路追踪提供者
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(cfg *config) {
		cfg.tracerProvider = tp
	}
}

// WithMeterProvider 设置指标提供者
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(cfg *config) {
		cfg.meterProvider = mp
	}
}

// WithLoggerProvider 设置日志提供者
func WithLoggerProvider(lp log.LoggerProvider) Option {
	return func(cfg *config) {
		cfg.loggerProvider = lp
	}
}

// Provider 基于 OTel 的可观测性统一提供者
type Provider struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	loggerProvider log.LoggerProvider
}

// New 创建 OTel Provider
func New(opts ...Option) *Provider {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	p := &Provider{
		tracerProvider: cfg.tracerProvider,
		meterProvider:  cfg.meterProvider,
		loggerProvider: cfg.loggerProvider,
	}

	// 默认使用 Noop 实现
	if p.tracerProvider == nil {
		p.tracerProvider = tracenoop.NewTracerProvider()
	}
	if p.meterProvider == nil {
		p.meterProvider = noop.NewMeterProvider()
	}

	return p
}

func (p *Provider) TracerProvider() trace.TracerProvider {
	return p.tracerProvider
}

func (p *Provider) MeterProvider() metric.MeterProvider {
	return p.meterProvider
}

func (p *Provider) LoggerProvider() log.LoggerProvider {
	return p.loggerProvider
}

func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	if tp, ok := p.tracerProvider.(interface{ Shutdown(context.Context) error }); ok {
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if mp, ok := p.meterProvider.(interface{ Shutdown(context.Context) error }); ok {
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if lp, ok := p.loggerProvider.(interface{ Shutdown(context.Context) error }); ok {
		if err := lp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
