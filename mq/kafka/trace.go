package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// kafkaMessageCarrier implements propagation.TextMapCarrier for sarama.ConsumerMessage headers
type kafkaMessageCarrier struct {
	headers []*sarama.RecordHeader
}

func newKafkaMessageCarrier(headers []*sarama.RecordHeader) *kafkaMessageCarrier {
	return &kafkaMessageCarrier{headers: headers}
}

func (c *kafkaMessageCarrier) Get(key string) string {
	for _, h := range c.headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaMessageCarrier) Set(key, value string) {
	var filtered []*sarama.RecordHeader
	for _, h := range c.headers {
		if string(h.Key) != key {
			filtered = append(filtered, h)
		}
	}
	filtered = append(filtered, &sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
	c.headers = filtered
}

func (c *kafkaMessageCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for _, h := range c.headers {
		keys = append(keys, string(h.Key))
	}
	return keys
}

// kafkaProducerCarrier implements propagation.TextMapCarrier for sarama.ProducerMessage headers
type kafkaProducerCarrier struct {
	msg *sarama.ProducerMessage
}

func (c *kafkaProducerCarrier) Get(key string) string {
	for i := range c.msg.Headers {
		if string(c.msg.Headers[i].Key) == key {
			return string(c.msg.Headers[i].Value)
		}
	}
	return ""
}

func (c *kafkaProducerCarrier) Set(key, value string) {
	c.msg.Headers = append(c.msg.Headers, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c *kafkaProducerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Headers))
	for i := range c.msg.Headers {
		keys = append(keys, string(c.msg.Headers[i].Key))
	}
	return keys
}

// propagateFromMessage 从 sarama.ConsumerMessage 的 headers 中提取 W3C traceparent，
// 注入到返回的 context 中。若无 trace context 则原样返回。
func propagateFromMessage(ctx context.Context, msg *sarama.ConsumerMessage) context.Context {
	carrier := newKafkaMessageCarrier(msg.Headers)
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// startConsumerSpan 为消费者消息创建 SpanKindConsumer span，
// 并将 trace context 从消息 headers 传播到返回的 context 中。
func startConsumerSpan(ctx context.Context, msg *sarama.ConsumerMessage) (context.Context, trace.Span) {
	ctx = propagateFromMessage(ctx, msg)
	tracer := telemetry.Tracer("mq.kafka.consumer")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("%s consume", msg.Topic),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", msg.Topic),
			attribute.Int("messaging.partition", int(msg.Partition)),
			attribute.Int64("messaging.offset", int64(msg.Offset)),
		),
		trace.WithSpanKind(trace.SpanKindConsumer),
	)
	return ctx, span
}

// injectProducerTrace 为生产者消息创建 SpanKindProducer span，
// 并将 trace context 注入到每条消息的 headers 中。
func injectProducerTrace(ctx context.Context, topic string, msgs []*sarama.ProducerMessage) (context.Context, trace.Span) {
	tracer := telemetry.Tracer("mq.kafka.producer")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("%s produce", topic),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
			attribute.Int("messaging.batch_size", len(msgs)),
		),
		trace.WithSpanKind(trace.SpanKindProducer),
	)

	// 将 trace context 注入到每条消息的 headers
	propagator := otel.GetTextMapPropagator()
	for _, msg := range msgs {
		carrier := &kafkaProducerCarrier{msg: msg}
		propagator.Inject(ctx, carrier)
	}

	return ctx, span
}
