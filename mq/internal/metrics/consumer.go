package metrics

import (
	"context"

	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/metric"
)

// ConsumerMetrics 消费者指标收集器
type ConsumerMetrics struct {
	consumeCounter    metric.Int64Counter
	retryCounter      metric.Int64Counter
	deadLetterCounter metric.Int64Counter
}

// NewConsumerMetrics 创建消费者指标收集器。
// prefix 用于区分不同 MQ 实现（如 "kafka"、"redis"、"httpsqs"），
// 会同时作为 meter 名称和指标名的前缀。
func NewConsumerMetrics(prefix string) *ConsumerMetrics {
	m := telemetry.Meter("github.com/gomooth/pkg/mq/" + prefix)
	consumeCounter, _ := m.Int64Counter(prefix+".consumer.messages", metric.WithDescription("Messages consumed successfully"))
	retryCounter, _ := m.Int64Counter(prefix+".consumer.retries", metric.WithDescription("Message retry attempts"))
	deadLetterCounter, _ := m.Int64Counter(prefix+".consumer.dead_letters", metric.WithDescription("Messages sent to dead letter"))
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
