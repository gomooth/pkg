package internal

import (
	"os"
	"time"

	"github.com/IBM/sarama"
)

// hostname 获取主机名，用于 sarama ClientID
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// BuildConsumerConfig 构建消费者专用 sarama.Config
func BuildConsumerConfig(timeout time.Duration) *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.ClientID = hostname()
	cfg.Net.DialTimeout = timeout

	// 消费者专用
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.Consumer.Offsets.AutoCommit.Enable = false

	return cfg
}

// BuildProducerConfig 构建生产者专用 sarama.Config
func BuildProducerConfig(timeout time.Duration) *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.ClientID = hostname()
	cfg.Net.DialTimeout = timeout

	// 生产者专用
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	cfg.Producer.Compression = sarama.CompressionZSTD
	cfg.Producer.Flush.Messages = 10
	cfg.Producer.Flush.Frequency = 500 * time.Millisecond
	cfg.Producer.Partitioner = sarama.NewHashPartitioner
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Retry.Backoff = 10 * time.Millisecond // 修复：原为 10（10ns），应为 10ms
	cfg.Producer.Timeout = timeout

	return cfg
}
