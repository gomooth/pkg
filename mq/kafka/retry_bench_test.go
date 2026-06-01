package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
)

// ============================================================
// 辅助
// ============================================================

// benchHandler 总是成功的 handler
type benchHandler struct{}

func (benchHandler) Handle(_ context.Context, _ string, _ []byte) error { return nil }

// benchFailHandler 总是失败的 handler
type benchFailHandler struct{}

func (benchFailHandler) Handle(_ context.Context, _ string, _ []byte) error {
	return errTestFail
}

var errTestFail = &testError{}

type testError struct{}

func (testError) Error() string { return "test error" }

// ============================================================
// P2: retry_item_convert — 重试项转换
// ============================================================

func BenchmarkToInternalRetryItem(b *testing.B) {
	b.ReportAllocs()

	item := &RetryItem{
		Topic:         "test-topic",
		Partition:     0,
		Offset:        100,
		Key:           []byte("key-1"),
		Value:         []byte("value-1"),
		Headers:       []HeaderKV{{Key: "trace-id", Value: []byte("abc-123")}},
		Attempt:       1,
		NextRetryAt:   time.Now().Add(5 * time.Second),
		ConsumerGroup: "test-group",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toInternalRetryItem(item)
	}
}

func BenchmarkToPublicRetryItem(b *testing.B) {
	b.ReportAllocs()

	item := &RetryItem{
		Topic:         "test-topic",
		Partition:     0,
		Offset:        100,
		Key:           []byte("key-1"),
		Value:         []byte("value-1"),
		Headers:       []HeaderKV{{Key: "trace-id", Value: []byte("abc-123")}},
		Attempt:       1,
		NextRetryAt:   time.Now().Add(5 * time.Second),
		ConsumerGroup: "test-group",
	}

	internal := toInternalRetryItem(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toPublicRetryItem(internal)
	}
}

func BenchmarkToInternalRetryItem_Batch(b *testing.B) {
	b.ReportAllocs()

	items := make([]*RetryItem, 100)
	for i := range items {
		items[i] = &RetryItem{
			Topic:         "test-topic",
			Partition:     int32(i % 4),
			Offset:        int64(i),
			Value:         []byte("value"),
			Attempt:       1,
			NextRetryAt:   time.Now().Add(5 * time.Second),
			ConsumerGroup: "test-group",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, item := range items {
			_ = toInternalRetryItem(item)
		}
	}
}

// ============================================================
// P2: saramaHeadersToPublic — Sarama 头部转换
// ============================================================

func BenchmarkSaramaHeadersToPublic(b *testing.B) {
	b.ReportAllocs()

	headers := []*sarama.RecordHeader{
		{Key: []byte("trace-id"), Value: []byte("abc-123-def-456")},
		{Key: []byte("span-id"), Value: []byte("span-789")},
		{Key: []byte("source"), Value: []byte("order-service")},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = saramaHeadersToPublic(headers)
	}
}

func BenchmarkSaramaHeadersToPublic_Empty(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = saramaHeadersToPublic(nil)
	}
}

// ============================================================
// P2: BackoffStrategy — 退避策略计算
// ============================================================

func BenchmarkExponentialDelay(b *testing.B) {
	b.ReportAllocs()

	strategy := &retry.ExponentialDelay{
		Base: time.Second,
		Max:  5 * time.Minute,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.Delay(0)
		_ = strategy.Delay(1)
		_ = strategy.Delay(2)
		_ = strategy.Delay(5)
	}
}

func BenchmarkFixedDelay(b *testing.B) {
	b.ReportAllocs()

	strategy := &retry.FixedDelay{Wait: time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.Delay(0)
		_ = strategy.Delay(5)
	}
}

func BenchmarkLinearDelay(b *testing.B) {
	b.ReportAllocs()

	strategy := &retry.LinearDelay{Base: time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.Delay(0)
		_ = strategy.Delay(1)
		_ = strategy.Delay(5)
	}
}

// ============================================================
// P2: RetryItem 构造开销
// ============================================================

func BenchmarkNewRetryItem(b *testing.B) {
	b.ReportAllocs()

	now := time.Now()
	backoff := &retry.FixedDelay{Wait: time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &RetryItem{
			Topic:         "orders",
			Partition:     0,
			Offset:        100,
			Key:           []byte("order-123"),
			Value:         []byte(`{"id":123}`),
			Headers:       []HeaderKV{{Key: "source", Value: []byte("api")}},
			Attempt:       1,
			NextRetryAt:   now.Add(backoff.Delay(0)),
			ConsumerGroup: "order-processor",
		}
	}
}
