package kafka

import (
	"context"
	"time"
)

// NewProducer 创建生产者实例
func NewProducer(brokers []string, opts ...ProducerOption) IProducer {
	cfg := producerConfig{
		timeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	engine := newProducerEngine(brokers, &cfg)
	return &producerImpl{engine: engine}
}

// producerImpl 生产者实现，包装 producerEngine 并实现 IProducer 接口
type producerImpl struct {
	engine *producerEngine
}

func (p *producerImpl) Start(ctx context.Context) error {
	return p.engine.Start(ctx)
}

func (p *producerImpl) Shutdown(ctx context.Context) error {
	return p.engine.Shutdown(ctx)
}

func (p *producerImpl) Produce(ctx context.Context, topic string, message []byte) error {
	return p.engine.Produce(ctx, topic, message)
}

func (p *producerImpl) ProduceBatch(ctx context.Context, topic string, messages ...[]byte) error {
	return p.engine.ProduceBatch(ctx, topic, messages...)
}

func (p *producerImpl) ProduceOrdered(ctx context.Context, topic string, partitionKey []byte, messages ...[]byte) error {
	return p.engine.ProduceOrdered(ctx, topic, partitionKey, messages...)
}
