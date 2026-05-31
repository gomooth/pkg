package redis

import "time"

// NewConsumer 创建消费者服务实例
func NewConsumer(addr string, opts ...ConsumerOption) IConsumeServer {
	cfg := consumerConfig{
		maxRetry:        3,
		emptyQueueSleep: time.Second,
		queuePrefix:     "queue:",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return newConsumerEngine(addr, &cfg)
}
