package internal

import (
	"time"

	"github.com/IBM/sarama"
)

// RetryItem 表示一个待重试的消息（sarama 层面内部表示）
type RetryItem struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   []HeaderKV

	Attempt     uint
	NextRetryAt time.Time

	ConsumerGroup string
}

// HeaderKV 简化的消息头键值对（sarama 原始 []byte 格式）
type HeaderKV struct {
	Key   []byte
	Value []byte
}

// SaramaMsgToRetryItem 将 sarama.ConsumerMessage 转换为 RetryItem
func SaramaMsgToRetryItem(msg *sarama.ConsumerMessage, consumerGroup string, attempt uint, nextRetryAt time.Time) *RetryItem {
	headers := make([]HeaderKV, len(msg.Headers))
	for i, h := range msg.Headers {
		headers[i] = HeaderKV{Key: h.Key, Value: h.Value}
	}

	return &RetryItem{
		Topic:         msg.Topic,
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		Key:           msg.Key,
		Value:         msg.Value,
		Headers:       headers,
		Attempt:       attempt,
		NextRetryAt:   nextRetryAt,
		ConsumerGroup: consumerGroup,
	}
}
