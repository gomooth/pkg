package metrics

import (
	"context"

	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/metric"
)

// ProducerMetrics 生产者指标收集器
type ProducerMetrics struct {
	produceCounter metric.Int64Counter
	errorCounter   metric.Int64Counter
}

// NewProducerMetrics 创建生产者指标收集器。
// prefix 用于区分不同 MQ 实现（如 "kafka"、"redis"）。
func NewProducerMetrics(prefix string) *ProducerMetrics {
	m := telemetry.Meter("github.com/gomooth/pkg/mq/" + prefix)
	produceCounter, _ := m.Int64Counter(prefix+".producer.messages", metric.WithDescription("Messages produced successfully"))
	errorCounter, _ := m.Int64Counter(prefix+".producer.errors", metric.WithDescription("Produce errors"))
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
