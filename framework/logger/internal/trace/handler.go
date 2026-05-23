package trace

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Injector 从 OTel context 中提取 trace_id 和 span_id 注入日志属性，
// 使本地日志可与 Jaeger/Grafana 等可观测性平台自动关联。
type Injector struct {
	Next slog.Handler
}

func (h *Injector) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Next.Enabled(ctx, level)
}

func (h *Injector) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Next.Handle(ctx, r)
}

func (h *Injector) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Injector{Next: h.Next.WithAttrs(attrs)}
}

func (h *Injector) WithGroup(name string) slog.Handler {
	return &Injector{Next: h.Next.WithGroup(name)}
}
