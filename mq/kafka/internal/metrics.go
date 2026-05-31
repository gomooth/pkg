package internal

import (
	"context"

	"github.com/gomooth/pkg/framework/metrics"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/gomooth/pkg/mq/kafka"

// ── Consumer 指标 ──

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
		consumeCounter:    m.Int64Counter("kafka.consumer.messages", metric.WithDescription("Messages consumed successfully")),
		retryCounter:      m.Int64Counter("kafka.consumer.retries", metric.WithDescription("Message retry attempts")),
		deadLetterCounter: m.Int64Counter("kafka.consumer.dead_letters", metric.WithDescription("Messages sent to dead letter")),
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

// ── Producer 指标 ──

// ProducerMetrics 生产者指标收集器
type ProducerMetrics struct {
	produceCounter metrics.Int64Counter
	errorCounter   metrics.Int64Counter
}

// NewProducerMetrics 创建生产者指标收集器
func NewProducerMetrics() *ProducerMetrics {
	m := metrics.GetProvider().Meter(meterName)
	return &ProducerMetrics{
		produceCounter: m.Int64Counter("kafka.producer.messages", metric.WithDescription("Messages produced successfully")),
		errorCounter:   m.Int64Counter("kafka.producer.errors", metric.WithDescription("Produce errors")),
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
