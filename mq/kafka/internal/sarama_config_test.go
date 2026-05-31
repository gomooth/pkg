package internal

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
)

func TestBuildConsumerConfig(t *testing.T) {
	cfg := BuildConsumerConfig(5 * time.Second)

	if cfg.Version != sarama.V3_6_0_0 {
		t.Errorf("expected version V3_6_0_0")
	}
	if cfg.Consumer.Offsets.AutoCommit.Enable {
		t.Errorf("expected auto commit disabled")
	}
	if cfg.Consumer.Offsets.Initial != sarama.OffsetNewest {
		t.Errorf("expected offset newest")
	}
	if len(cfg.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Errorf("expected 1 rebalance strategy")
	}
}

func TestBuildProducerConfig(t *testing.T) {
	cfg := BuildProducerConfig(10 * time.Second)

	if cfg.Version != sarama.V3_6_0_0 {
		t.Errorf("expected version V3_6_0_0")
	}
	if cfg.Producer.RequiredAcks != sarama.WaitForAll {
		t.Errorf("expected WaitForAll")
	}
	if cfg.Producer.Compression != sarama.CompressionZSTD {
		t.Errorf("expected ZSTD compression")
	}
	if cfg.Producer.Timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", cfg.Producer.Timeout)
	}
	// 修复：HashPartitioner 替代 RoundRobin
	// Partitioner 是 PartitionerConstructor (func 类型)，不能直接做类型断言；
	// 通过行为验证：相同 key 多次分区应返回相同结果
	p := cfg.Producer.Partitioner("test-topic")
	msg1 := &sarama.ProducerMessage{Key: sarama.StringEncoder("my-key")}
	msg2 := &sarama.ProducerMessage{Key: sarama.StringEncoder("my-key")}
	part1, err := p.Partition(msg1, 10)
	if err != nil {
		t.Fatalf("partition failed: %v", err)
	}
	part2, err := p.Partition(msg2, 10)
	if err != nil {
		t.Fatalf("partition failed: %v", err)
	}
	if part1 != part2 {
		t.Errorf("HashPartitioner should return same partition for same key, got %d and %d", part1, part2)
	}
	// 修复：10ms 而非 10ns
	if cfg.Producer.Retry.Backoff != 10*time.Millisecond {
		t.Errorf("expected 10ms retry backoff, got %v", cfg.Producer.Retry.Backoff)
	}
}
