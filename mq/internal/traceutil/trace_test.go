package traceutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestExtractTraceContext_NonJSON(t *testing.T) {
	ctx := context.Background()
	result := ExtractTraceContext(ctx, "not json")
	assert.Equal(t, ctx, result)
}

func TestExtractTraceContext_NoTraceContext(t *testing.T) {
	ctx := context.Background()
	msg := `{"key": "value"}`
	result := ExtractTraceContext(ctx, msg)
	assert.Equal(t, ctx, result)
}

func TestExtractTraceContext_InvalidTraceParent(t *testing.T) {
	ctx := context.Background()
	msg := `{"__trace_context": 123}`
	result := ExtractTraceContext(ctx, msg)
	assert.Equal(t, ctx, result)
}

func TestInjectTraceContext_NonJSON(t *testing.T) {
	ctx := context.Background()
	msg := "not json"
	result := InjectTraceContext(ctx, msg)
	assert.Equal(t, msg, result)
}

func TestInjectAndExtract(t *testing.T) {
	// 设置 propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 使用 SDK TracerProvider 创建有效的 span context
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// 注入
	msg := `{"key": "value"}`
	injected := InjectTraceContext(ctx, msg)
	require.NotEqual(t, msg, injected)

	// 提取
	extractedCtx := ExtractTraceContext(context.Background(), injected)
	extractedSpan := trace.SpanFromContext(extractedCtx)
	require.True(t, extractedSpan.SpanContext().IsValid())
	assert.Equal(t, span.SpanContext().TraceID(), extractedSpan.SpanContext().TraceID())
}

func TestInjectTraceContext_Overwrite(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	ctx, _ := tracer.Start(context.Background(), "test-span")

	msg := `{"key": "value", "__trace_context": "\"old-traceparent\""}`
	injected := InjectTraceContext(ctx, msg)
	assert.Contains(t, injected, `"__trace_context"`)
	assert.NotContains(t, injected, "old-traceparent")
}
