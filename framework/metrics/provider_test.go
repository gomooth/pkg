package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestDefaultProvider_IsNoop(t *testing.T) {
	p := GetProvider()
	assert.NotNil(t, p)

	m := p.Meter("test")
	assert.NotNil(t, m)

	counter := m.Int64Counter("test.counter")
	assert.NotNil(t, counter)

	histogram := m.Int64Histogram("test.histogram")
	assert.NotNil(t, histogram)

	fhistogram := m.Float64Histogram("test.float_histogram")
	assert.NotNil(t, fhistogram)

	igauge := m.Int64Gauge("test.int64_gauge")
	assert.NotNil(t, igauge)

	fgauge := m.Float64Gauge("test.float64_gauge")
	assert.NotNil(t, fgauge)
}

func TestSetProvider_NilRestoresNoop(t *testing.T) {
	original := GetProvider()
	SetProvider(nil)
	assert.NotNil(t, GetProvider())
	SetProvider(original)
}

func TestOtelProvider_LateBinding(t *testing.T) {
	// 模拟包级变量初始化场景：在 otel.SetMeterProvider 之前创建指标仪器
	m := GetProvider().Meter("test.late_binding")
	counter := m.Int64Counter("late.counter")
	histogram := m.Float64Histogram("late.histogram")
	igauge := m.Int64Gauge("late.int64_gauge")
	fgauge := m.Float64Gauge("late.float64_gauge")

	// 此时 OTel 尚未配置真实 Provider，调用不 panic 即可
	counter.Add(t.Context(), 1)
	histogram.Record(t.Context(), 0.5)
	igauge.Record(t.Context(), 10)
	fgauge.Record(t.Context(), 0.5)

	// 配置真实 OTel MeterProvider
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(otel.GetMeterProvider()) })

	// 延迟绑定生效：之前创建的仪器现在写入真实 Provider
	counter.Add(t.Context(), 2)
	histogram.Record(t.Context(), 1.5)
	igauge.Record(t.Context(), 42)
	fgauge.Record(t.Context(), 3.14)

	// 验证数据确实被采集
	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(t.Context(), &rm))

	foundCounter := false
	foundHistogram := false
	foundInt64Gauge := false
	foundFloat64Gauge := false
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name == "test.late_binding" {
			for _, m := range sm.Metrics {
				switch m.Name {
				case "late.counter":
					foundCounter = true
				case "late.histogram":
					foundHistogram = true
				case "late.int64_gauge":
					foundInt64Gauge = true
				case "late.float64_gauge":
					foundFloat64Gauge = true
				}
			}
		}
	}
	assert.True(t, foundCounter, "late.counter should be collected after late binding")
	assert.True(t, foundHistogram, "late.histogram should be collected after late binding")
	assert.True(t, foundInt64Gauge, "late.int64_gauge should be collected after late binding")
	assert.True(t, foundFloat64Gauge, "late.float64_gauge should be collected after late binding")
}
