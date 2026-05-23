package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Provider 指标提供者接口
type Provider interface {
	Meter(name string, opts ...metric.MeterOption) Meter
}

// Meter 指标计量器接口
type Meter interface {
	Int64Counter(name string, opts ...metric.Int64CounterOption) Int64Counter
	Int64Histogram(name string, opts ...metric.Int64HistogramOption) Int64Histogram
	Float64Histogram(name string, opts ...metric.Float64HistogramOption) Float64Histogram
	Int64Gauge(name string, opts ...metric.Int64GaugeOption) Int64Gauge
	Float64Gauge(name string, opts ...metric.Float64GaugeOption) Float64Gauge
}

// Int64Counter 整数计数器接口
type Int64Counter interface {
	Add(ctx context.Context, incr int64, opts ...metric.AddOption)
}

// Int64Histogram 整数直方图接口
type Int64Histogram interface {
	Record(ctx context.Context, value int64, opts ...metric.RecordOption)
}

// Float64Histogram 浮点直方图接口
type Float64Histogram interface {
	Record(ctx context.Context, value float64, opts ...metric.RecordOption)
}

// Int64Gauge 整数仪表盘接口，记录瞬时值（如当前活跃连接数）
type Int64Gauge interface {
	Record(ctx context.Context, value int64, opts ...metric.RecordOption)
}

// Float64Gauge 浮点仪表盘接口，记录瞬时值（如当前温度、内存使用率）
type Float64Gauge interface {
	Record(ctx context.Context, value float64, opts ...metric.RecordOption)
}

// otelProvider 委托 OTel 全局 MeterProvider，原生支持延迟绑定：
// 即使 Meter/Counter 在 otel.SetMeterProvider() 之前创建，
// 一旦设置真实 Provider，所有已创建的指标仪器自动生效。
type otelProvider struct{}

func (p *otelProvider) Meter(name string, opts ...metric.MeterOption) Meter {
	return &otelMeter{inner: otel.GetMeterProvider().Meter(name, opts...)}
}

type otelMeter struct {
	inner metric.Meter
}

func (m *otelMeter) Int64Counter(name string, opts ...metric.Int64CounterOption) Int64Counter {
	c, _ := m.inner.Int64Counter(name, opts...)
	return &otelInt64Counter{inner: c}
}

func (m *otelMeter) Int64Histogram(name string, opts ...metric.Int64HistogramOption) Int64Histogram {
	h, _ := m.inner.Int64Histogram(name, opts...)
	return &otelInt64Histogram{inner: h}
}

func (m *otelMeter) Float64Histogram(name string, opts ...metric.Float64HistogramOption) Float64Histogram {
	h, _ := m.inner.Float64Histogram(name, opts...)
	return &otelFloat64Histogram{inner: h}
}

func (m *otelMeter) Int64Gauge(name string, opts ...metric.Int64GaugeOption) Int64Gauge {
	g, _ := m.inner.Int64Gauge(name, opts...)
	return &otelInt64Gauge{inner: g}
}

func (m *otelMeter) Float64Gauge(name string, opts ...metric.Float64GaugeOption) Float64Gauge {
	g, _ := m.inner.Float64Gauge(name, opts...)
	return &otelFloat64Gauge{inner: g}
}

type otelInt64Counter struct {
	inner metric.Int64Counter
}

func (c *otelInt64Counter) Add(ctx context.Context, incr int64, opts ...metric.AddOption) {
	c.inner.Add(ctx, incr, opts...)
}

type otelInt64Histogram struct {
	inner metric.Int64Histogram
}

func (h *otelInt64Histogram) Record(ctx context.Context, value int64, opts ...metric.RecordOption) {
	h.inner.Record(ctx, value, opts...)
}

type otelFloat64Histogram struct {
	inner metric.Float64Histogram
}

func (h *otelFloat64Histogram) Record(ctx context.Context, value float64, opts ...metric.RecordOption) {
	h.inner.Record(ctx, value, opts...)
}

type otelInt64Gauge struct {
	inner metric.Int64Gauge
}

func (g *otelInt64Gauge) Record(ctx context.Context, value int64, opts ...metric.RecordOption) {
	g.inner.Record(ctx, value, opts...)
}

type otelFloat64Gauge struct {
	inner metric.Float64Gauge
}

func (g *otelFloat64Gauge) Record(ctx context.Context, value float64, opts ...metric.RecordOption) {
	g.inner.Record(ctx, value, opts...)
}

// 全局 Provider，默认委托 OTel 全局 MeterProvider
var globalProvider Provider = &otelProvider{}

// SetProvider 设置全局 Metrics Provider。
// 传入 nil 重置为默认的 OTel 委托 Provider（未配置时零开销）。
// 注意：此函数应在应用启动时调用，晚于包级变量初始化，
// 因此仅对后续新创建的 Meter 生效；推荐直接使用 otel.SetMeterProvider()。
func SetProvider(p Provider) {
	if p != nil {
		globalProvider = p
	} else {
		globalProvider = &otelProvider{}
	}
}

// GetProvider 获取全局 Metrics Provider
func GetProvider() Provider {
	return globalProvider
}

// Attribute 辅助函数
func Attr(key, val string) attribute.KeyValue {
	return attribute.String(key, val)
}

func AttrInt(key string, val int) attribute.KeyValue {
	return attribute.Int(key, val)
}

func AttrFloat(key string, val float64) attribute.KeyValue {
	return attribute.Float64(key, val)
}

func AttrBool(key string, val bool) attribute.KeyValue {
	return attribute.Bool(key, val)
}
