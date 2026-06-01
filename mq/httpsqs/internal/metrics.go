package internal

import (
	"context"

	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/gomooth/pkg/mq/httpsqs"

// ConsumerMetrics 消费者指标收集器
type ConsumerMetrics struct {
	consumeCounter    metric.Int64Counter
	retryCounter      metric.Int64Counter
	deadLetterCounter metric.Int64Counter
}

// NewConsumerMetrics 创建消费者指标收集器
func NewConsumerMetrics() *ConsumerMetrics {
	m := telemetry.Meter(meterName)
	consumeCounter, _ := m.Int64Counter("httpsqs.consumer.messages", metric.WithDescription("Messages consumed successfully"))
	retryCounter, _ := m.Int64Counter("httpsqs.consumer.retries", metric.WithDescription("Message retry attempts"))
	deadLetterCounter, _ := m.Int64Counter("httpsqs.consumer.dead_letters", metric.WithDescription("Messages sent to dead letter"))
	return &ConsumerMetrics{
		consumeCounter:    consumeCounter,
		retryCounter:      retryCounter,
		deadLetterCounter: deadLetterCounter,
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
