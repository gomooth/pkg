// Package traceutil 提供消息队列的通用 OTel 链路追踪注入与提取工具函数。
package traceutil

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TraceContextKey JSON 消息体中存储 trace context 的字段名
const TraceContextKey = "__trace_context"

// ExtractTraceContext 从消息中提取 trace context。
// 仅对 JSON 格式消息有效，非 JSON 消息原样返回原始 context。
func ExtractTraceContext(ctx context.Context, msg string) context.Context {
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(msg), &body); err != nil {
		return ctx
	}

	raw, ok := body[TraceContextKey]
	if !ok {
		return ctx
	}

	var traceParent string
	if err := json.Unmarshal(raw, &traceParent); err != nil {
		return ctx
	}

	carrier := propagation.MapCarrier{"traceparent": traceParent}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// InjectTraceContext 向消息中注入 trace context。
// 仅对 JSON 格式消息有效，非 JSON 消息原样返回。
func InjectTraceContext(ctx context.Context, msg string) string {
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(msg), &body); err != nil {
		return msg
	}

	// 删除已有的 trace context
	delete(body, TraceContextKey)

	// 注入新的 trace context
	carrier := make(propagation.MapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	if tp, ok := carrier["traceparent"]; ok {
		raw, _ := json.Marshal(tp)
		body[TraceContextKey] = raw
	}

	result, _ := json.Marshal(body)
	return string(result)
}
