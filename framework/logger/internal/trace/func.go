package trace

import (
	"context"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func GetSpanInfo(ctx context.Context) (traceID, spanID string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return "", ""
	}

	sc := span.SpanContext()
	return sc.TraceID().String(), sc.SpanID().String()
}

func GetSpanContext(ctx context.Context) (trace.SpanContext, bool) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return trace.SpanContext{}, false
	}
	return span.SpanContext(), true
}

func GetBaggage(ctx context.Context, key string) (string, bool) {
	bag := baggage.FromContext(ctx)
	member := bag.Member(key)
	if len(member.Key()) == 0 {
		return "", false
	}
	return member.Value(), true
}

func SetBaggage(ctx context.Context, key, value string) (context.Context, error) {
	member, err := baggage.NewMember(key, value)
	if err != nil {
		return ctx, err
	}
	bag := baggage.FromContext(ctx)
	bag, err = bag.SetMember(member)
	if err != nil {
		return ctx, err
	}
	return baggage.ContextWithBaggage(ctx, bag), nil
}
