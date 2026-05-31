package kafka

import (
	"github.com/gomooth/pkg/mq/kafka/internal"
)

// toInternalRetryItem 将公开 RetryItem 转换为内部 RetryItem
func toInternalRetryItem(item *RetryItem) *internal.RetryItem {
	if item == nil {
		return nil
	}
	headers := make([]internal.HeaderKV, len(item.Headers))
	for i, h := range item.Headers {
		headers[i] = internal.HeaderKV{Key: []byte(h.Key), Value: h.Value}
	}
	return &internal.RetryItem{
		Topic:         item.Topic,
		Partition:     item.Partition,
		Offset:        item.Offset,
		Key:           item.Key,
		Value:         item.Value,
		Headers:       headers,
		Attempt:       uint(item.Attempt),
		NextRetryAt:   item.NextRetryAt,
		ConsumerGroup: item.ConsumerGroup,
	}
}

// toPublicRetryItem 将内部 RetryItem 转换为公开 RetryItem
func toPublicRetryItem(item *internal.RetryItem) *RetryItem {
	if item == nil {
		return nil
	}
	headers := make([]HeaderKV, len(item.Headers))
	for i, h := range item.Headers {
		headers[i] = HeaderKV{Key: string(h.Key), Value: h.Value}
	}
	return &RetryItem{
		Topic:         item.Topic,
		Partition:     item.Partition,
		Offset:        item.Offset,
		Key:           item.Key,
		Value:         item.Value,
		Headers:       headers,
		Attempt:       int(item.Attempt),
		NextRetryAt:   item.NextRetryAt,
		ConsumerGroup: item.ConsumerGroup,
	}
}

// toPublicRetryItems 批量转换
func toPublicRetryItems(items []*internal.RetryItem) []*RetryItem {
	result := make([]*RetryItem, len(items))
	for i, item := range items {
		result[i] = toPublicRetryItem(item)
	}
	return result
}
