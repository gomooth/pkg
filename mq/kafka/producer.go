package kafka

import (
	"context"
	"time"

	"github.com/gomooth/pkg/mq/internal/types"
)

// NewProducer 创建生产者实例
func NewProducer(brokers []string, opts ...ProducerOption) types.IProducer {
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

// 编译时接口检查
var _ types.IProducer = (*producerImpl)(nil)

func (p *producerImpl) Start(ctx context.Context) error {
	return p.engine.Start(ctx)
}

func (p *producerImpl) Shutdown(ctx context.Context) error {
	return p.engine.Shutdown(ctx)
}

func (p *producerImpl) Produce(ctx context.Context, dest string, message []byte, opts ...types.ProduceOption) error {
	return p.engine.Produce(ctx, dest, message, opts...)
}

func (p *producerImpl) ProduceBatch(ctx context.Context, dest string, messages [][]byte, opts ...types.ProduceOption) error {
	return p.engine.ProduceBatch(ctx, dest, messages, opts...)
}