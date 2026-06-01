package internal

import (
	"context"

	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/gomooth/pkg/mq/kafka"

// ConsumerMetrics 消费者指标收集器
type ConsumerMetrics struct {
	consumeCounter    metric.Int64Counter
	retryCounter      metric.Int64Counter
	deadLetterCounter metric.Int64Counter
}

// NewConsumerMetrics 创建消费者指标收集器
func NewConsumerMetrics() *ConsumerMetrics {
	m := telemetry.Meter(meterName)
	consumeCounter, _ := m.Int64Counter("kafka.consumer.messages", metric.WithDescription("Messages consumed successfully"))
	retryCounter, _ := m.Int64Counter("kafka.consumer.retries", metric.WithDescription("Message retry attempts"))
	deadLetterCounter, _ := m.Int64Counter("kafka.consumer.dead_letters", metric.WithDescription("Messages sent to dead letter"))
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

// ProducerMetrics 生产者指标收集器
type ProducerMetrics struct {
	produceCounter metric.Int64Counter
	errorCounter   metric.Int64Counter
}

// NewProducerMetrics 创建生产者指标收集器
func NewProducerMetrics() *ProducerMetrics {
	m := telemetry.Meter(meterName)
	produceCounter, _ := m.Int64Counter("kafka.producer.messages", metric.WithDescription("Messages produced successfully"))
	errorCounter, _ := m.Int64Counter("kafka.producer.errors", metric.WithDescription("Produce errors"))
	return &ProducerMetrics{
		produceCounter: produceCounter,
		errorCounter:   errorCounter,
	}
}

func (m *ProducerMetrics) OnProduce(count int) {
	if m != nil && m.produceCounter != nil {
		m.produceCounter.Add(context.Background(), int64(count))
	}
}

func (m *ProducerMetrics) OnError() {
	if m != nil && m.errorCounter != nil {
		m.errorCounter.Add(context.Background(), 1)
	}
}
