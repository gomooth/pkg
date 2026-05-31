package internal

import (
	"context"

	"github.com/gomooth/pkg/framework/metrics"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/gomooth/pkg/mq/httpsqs"

// ConsumerMetrics 消费者指标收集器
type ConsumerMetrics struct {
	consumeCounter    metrics.Int64Counter
	retryCounter      metrics.Int64Counter
	deadLetterCounter metrics.Int64Counter
}

// NewConsumerMetrics 创建消费者指标收集器
func NewConsumerMetrics() *ConsumerMetrics {
	m := metrics.GetProvider().Meter(meterName)
	return &ConsumerMetrics{
		consumeCounter:    m.Int64Counter("httpsqs.consumer.messages", metric.WithDescription("Messages consumed successfully")),
		retryCounter:      m.Int64Counter("httpsqs.consumer.retries", metric.WithDescription("Message retry attempts")),
		deadLetterCounter: m.Int64Counter("httpsqs.consumer.dead_letters", metric.WithDescription("Messages sent to dead letter")),
	}
}

func (m *ConsumerMetrics) OnConsume() {
	if m != nil && m.consumeCounter != nil {
		m.consumeCounter.Add(context.Background(), 1)
	}
}

func (m *ConsumerMetrics) OnRetry() {
	if m != nil && m.retryCounter != nil {
		m.retryCounter.Add(context.Background(), 1)
	}
}

func (m *ConsumerMetrics) OnDeadLetter() {
	if m != nil && m.deadLetterCounter != nil {
		m.deadLetterCounter.Add(context.Background(), 1)
	}
}
