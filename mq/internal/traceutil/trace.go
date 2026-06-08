// Package traceutil 提供消息队列的通用 OTel 链路追踪注入与提取工具函数。
package traceutil

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// tracePropagationKeys W3C TraceContext 标准传播字段名
var tracePropagationKeys = []string{"traceparent", "tracestate", "baggage"}

// ExtractTraceContext 从消息中提取 trace context。
// 仅对 JSON 格式消息有效，非 JSON 消息原样返回原始 context。
func ExtractTraceContext(ctx context.Context, msg string) context.Context {
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(msg), &body); err != nil {
		return ctx
	}

	// 读取 W3C 传播字段
	carrier := make(propagation.MapCarrier)
	for _, key := range tracePropagationKeys {
		if raw, ok := body[key]; ok {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				continue
			}
			carrier.Set(key, value)
		}
	}

	// 至少需要 traceparent 才有意义
	if _, ok := carrier["traceparent"]; !ok {
		return ctx
	}

	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// InjectTraceContext 向消息中注入 trace context。
// 仅对 JSON 格式消息有效，非 JSON 消息原样返回。
func InjectTraceContext(ctx context.Context, msg string) string {
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(msg), &body); err != nil {
		return msg
	}

	// 删除已有的传播字段（包括旧格式 __trace_context）
	for _, key := range append(tracePropagationKeys, "__trace_context") {
		delete(body, key)
	}

	// 注入新的 trace context
	carrier := make(propagation.MapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	for _, key := range carrier.Keys() {
		raw, _ := json.Marshal(carrier.Get(key))
		body[key] = raw
	}

	result, _ := json.Marshal(body)
	return string(result)
}
