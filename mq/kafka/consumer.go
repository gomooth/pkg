package kafka

import (
	"time"

	"github.com/gomooth/pkg/mq/internal/types"
)

// NewConsumer 创建消费者服务实例
func NewConsumer(brokers []string, opts ...ConsumerOption) types.IConsumeServer {
	cfg := consumerConfig{
		timeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return newConsumerEngine(brokers, &cfg)
}