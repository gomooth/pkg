package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestNoopProvider_Default(t *testing.T) {
	p := GetProvider()
	require.NotNil(t, p)

	// 默认 noop，不 panic
	tracer := Tracer("test")
	require.NotNil(t, tracer)

	meter := Meter("test")
	require.NotNil(t, meter)

	// Shutdown 不报错
	assert.NoError(t, p.Shutdown(context.Background()))

	// LoggerProvider 为 nil
	assert.Nil(t, p.LoggerProvider())
}

func TestSetProvider_Nil(t *testing.T) {
	original := GetProvider()
	defer SetProvider(original)

	SetProvider(nil)
	p := GetProvider()
	require.NotNil(t, p)
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_WithOptions(t *testing.T) {
	tp := tracenoop.NewTracerProvider()
	mp := metricnoop.NewMeterProvider()

	p := NewProvider(
		WithTracerProvider(tp),
		WithMeterProvider(mp),
	)
	require.NotNil(t, p)

	assert.Equal(t, tp, p.TracerProvider())
	assert.Equal(t, mp, p.MeterProvider())
	assert.Nil(t, p.LoggerProvider())
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestTracer_ReturnsOtelTracer(t *testing.T) {
	tracer := Tracer("test")
	require.NotNil(t, tracer)

	// Noop tracer 可以创建 span
	_, span := tracer.Start(context.Background(), "test-span")
	require.NotNil(t, span)
	span.End()
}

func TestMeter_ReturnsOtelMeter(t *testing.T) {
	meter := Meter("test")
	require.NotNil(t, meter)

	// Noop meter 可以创建 counter
	counter, err := meter.Int64Counter("test.counter")
	assert.NoError(t, err)
	assert.NotNil(t, counter)
}
