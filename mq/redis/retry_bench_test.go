package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/redis/go-redis/v9"
)

// ============================================================
// 辅助
// ============================================================

type benchSuccessHandler struct{}

func (benchSuccessHandler) Handle(_ context.Context, _ string, _ []byte) error {
	return nil
}

type benchFailHandler struct{}

func (benchFailHandler) Handle(_ context.Context, _ string, _ []byte) error {
	return errBenchFail
}

var errBenchFail = &benchError{}

type benchError struct{}

func (benchError) Error() string { return "bench error" }

func newBenchRedisClient(b *testing.B) *redis.Client {
	b.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

// ============================================================
// P2: syncRetryStrategy.OnMessage — 成功路径（无重试）
// ============================================================

func BenchmarkSyncRetry_OnMessage_Success(b *testing.B) {
	b.ReportAllocs()

	strategy := newSyncRetryStrategy(
		benchSuccessHandler{},
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		nil,
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.OnMessage(context.Background(), "test-queue", []byte("hello"))
	}
}

// ============================================================
// P2: syncRetryStrategy.OnMessage — 失败路径（零重试）
// ============================================================

func BenchmarkSyncRetry_OnMessage_Fail_NoRetry(b *testing.B) {
	b.ReportAllocs()

	strategy := newSyncRetryStrategy(
		benchFailHandler{},
		0, // no retry
		&retry.FixedDelay{Wait: time.Millisecond},
		nil,
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.OnMessage(context.Background(), "test-queue", []byte("hello"))
	}
}

// ============================================================
// P2: requeueRetryStrategy.OnMessage — 成功路径
// ============================================================

func BenchmarkRequeueRetry_OnMessage_Success(b *testing.B) {
	b.ReportAllocs()

	client := newBenchRedisClient(b)
	defer client.Close()

	strategy := newRequeueRetryStrategy(
		benchSuccessHandler{},
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"queue:",
		nil,
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.OnMessage(context.Background(), "test-queue", []byte("hello"))
	}
}

// ============================================================
// P2: requeueRetryStrategy.OnMessage — 失败后入队
// ============================================================

func BenchmarkRequeueRetry_OnMessage_Fail_Requeue(b *testing.B) {
	b.ReportAllocs()

	client := newBenchRedisClient(b)
	defer client.Close()

	strategy := newRequeueRetryStrategy(
		benchFailHandler{},
		3,
		&retry.FixedDelay{Wait: time.Millisecond},
		client,
		"queue:",
		nil,
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 每次使用不同的消息，避免 AttemptTracker 重复 key
		data := []byte(time.Now().String())
		_ = strategy.OnMessage(context.Background(), "test-queue", data)
	}
}

// ============================================================
// P2: AttemptTracker 操作 — 通过 requeueRetryStrategy 间接测试
// 直接基准测试通过 internal 包的导出函数进行
// ============================================================

// Note: AttemptTracker is in the internal package and cannot be directly
// benchmarked from outside. The tracker operations are covered indirectly
// through the requeueRetryStrategy benchmarks above.
// For direct benchmarking, see mq/redis/internal/ tests.

// ============================================================
// P2: BackoffStrategy — 退避策略计算
// ============================================================

func BenchmarkRedisExponentialDelay(b *testing.B) {
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

func BenchmarkRedisFixedDelay(b *testing.B) {
	b.ReportAllocs()

	strategy := &retry.FixedDelay{Wait: time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.Delay(0)
		_ = strategy.Delay(5)
	}
}

// ============================================================
// P2: 选项构造开销
// ============================================================

func BenchmarkWithMaxRetry(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithMaxRetry(3)
	}
}

func BenchmarkWithBackoff(b *testing.B) {
	b.ReportAllocs()

	backoff := &retry.ExponentialDelay{Base: time.Second, Max: 5 * time.Minute}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithBackoff(backoff)
	}
}

func BenchmarkWithRetryMode(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithRetryMode(RetryModeSync)
	}
}

func BenchmarkWithHandlerTimeout(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithHandlerTimeout(30 * time.Second)
	}
}
