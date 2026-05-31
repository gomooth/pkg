package redis

import "context"

// NewProducer 创建生产者实例
func NewProducer(addr string, opts ...ProducerOption) IProducer {
	cfg := producerConfig{
		queuePrefix: "queue:",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	engine := newProducerEngine(addr, &cfg)
	return &producerImpl{engine: engine}
}

// producerImpl 生产者实现，包装 producerEngine 并实现 IProducer 接口
type producerImpl struct {
	engine *producerEngine
}

// 编译时接口检查
var _ IProducer = (*producerImpl)(nil)

func (p *producerImpl) Start(ctx context.Context) error {
	return p.engine.Start(ctx)
}

func (p *producerImpl) Shutdown(ctx context.Context) error {
	return p.engine.Shutdown(ctx)
}

func (p *producerImpl) Produce(ctx context.Context, queue string, message []byte) error {
	return p.engine.Produce(ctx, queue, message)
}

func (p *producerImpl) ProduceBatch(ctx context.Context, queue string, messages ...[]byte) error {
	return p.engine.ProduceBatch(ctx, queue, messages...)
}
