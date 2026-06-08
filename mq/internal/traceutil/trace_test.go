package traceutil

import (
	"context"
	"encoding/json"
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
	msg := `{"traceparent": 123}`
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

	// 验证使用 W3C 标准字段名
	assert.Contains(t, injected, `"traceparent"`)
	assert.NotContains(t, injected, `"__trace_context"`)

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

	msg := `{"key": "value", "traceparent": "00-old-old-old-old-01"}`
	injected := InjectTraceContext(ctx, msg)
	assert.Contains(t, injected, `"traceparent"`)
	assert.NotContains(t, injected, "00-old-old-old-old-01")
}

func TestExtractTraceContext_WithTraceState(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// 注入并手动添加 tracestate
	msg := `{"key": "value"}`
	injected := InjectTraceContext(ctx, msg)
	assert.Contains(t, injected, `"traceparent"`)

	// 手动注入 tracestate 到 JSON 再提取
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(injected), &body))
	tsRaw, _ := json.Marshal("vendor1=abc")
	body["tracestate"] = tsRaw
	modified, _ := json.Marshal(body)

	// 提取应成功
	extractedCtx := ExtractTraceContext(context.Background(), string(modified))
	extractedSpan := trace.SpanFromContext(extractedCtx)
	require.True(t, extractedSpan.SpanContext().IsValid())
	assert.Equal(t, span.SpanContext().TraceID(), extractedSpan.SpanContext().TraceID())
}

func TestExtractTraceContext_WithBaggage(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// 注入 traceparent 后，手动添加 baggage 字段
	msg := `{"key": "value"}`
	injected := InjectTraceContext(ctx, msg)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(injected), &body))
	bagRaw, _ := json.Marshal("env=prod")
	body["baggage"] = bagRaw
	modified, _ := json.Marshal(body)

	// 提取应成功且包含 baggage
	extractedCtx := ExtractTraceContext(context.Background(), string(modified))
	extractedSpan := trace.SpanFromContext(extractedCtx)
	require.True(t, extractedSpan.SpanContext().IsValid())

	// 验证 baggage 传播：通过 propagator 从提取的 context 注入到新 carrier
	newCarrier := make(propagation.MapCarrier)
	otel.GetTextMapPropagator().Inject(extractedCtx, newCarrier)
	assert.Equal(t, "env=prod", newCarrier.Get("baggage"))
}

func TestInjectTraceContext_CleansOldFormat(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	ctx, _ := tracer.Start(context.Background(), "test-span")

	// 消息包含旧格式 __trace_context 字段
	msg := `{"key": "value", "__trace_context": "\"00-old-old-old-old-01\""}`
	injected := InjectTraceContext(ctx, msg)

	// 旧格式字段应被清除
	assert.NotContains(t, injected, `"__trace_context"`)
	// 新格式字段应存在
	assert.Contains(t, injected, `"traceparent"`)
}
