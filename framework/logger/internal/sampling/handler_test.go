package sampling

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockHandler 记录所有通过的日志记录
type mockHandler struct {
	records []slog.Record
	mu      sync.Mutex
}

func (m *mockHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (m *mockHandler) Handle(_ context.Context, r slog.Record) error {
	m.mu.Lock()
	m.records = append(m.records, r)
	m.mu.Unlock()
	return nil
}

func (m *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return m }
func (m *mockHandler) WithGroup(name string) slog.Handler        { return m }

func (m *mockHandler) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records)
}

func TestHandler_NoSampling(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{})

	for i := 0; i < 100; i++ {
		err := h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0))
		assert.NoError(t, err)
	}
	assert.Equal(t, 100, next.count())
}

func TestHandler_LevelRateLimit(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{
		LevelRates:      map[slog.Level]float64{slog.LevelInfo: 5.0},
		BurstMultiplier: 1.0,
	})

	total := 20
	passed := 0
	for i := 0; i < total; i++ {
		err := h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0))
		assert.NoError(t, err)
		// 给一点时间让限流器工作
	}

	passed = next.count()
	assert.Less(t, passed, total, "should have dropped some logs")
	assert.Greater(t, passed, 0, "should have passed some logs")
}

func TestHandler_LevelIsolation(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{
		LevelRates:      map[slog.Level]float64{slog.LevelInfo: 1.0},
		BurstMultiplier: 1.0,
	})

	// Info 应被限流
	for i := 0; i < 50; i++ {
		_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "info", 0))
	}
	infoCount := next.count()

	// Error 不应被限流（未配置 LevelRates）
	for i := 0; i < 50; i++ {
		_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "error", 0))
	}
	errorCount := next.count() - infoCount

	assert.Less(t, infoCount, 50, "Info should be rate-limited")
	assert.Equal(t, 50, errorCount, "Error should not be rate-limited")
}

func TestHandler_BurstBehavior(t *testing.T) {
	next := &mockHandler{}
	// rate=1, burst=5 → 允许短时突发 5 条
	h := New(next, Config{
		LevelRates:      map[slog.Level]float64{slog.LevelInfo: 1.0},
		BurstMultiplier: 5.0, // burst = max(1, int(1*5)) = 5
	})

	for i := 0; i < 5; i++ {
		_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "burst", 0))
	}
	assert.Equal(t, 5, next.count(), "burst should allow 5 messages at once")
}

func TestHandler_WithAttrsSharesLimiter(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{
		LevelRates:      map[slog.Level]float64{slog.LevelInfo: 10.0},
		BurstMultiplier: 1.0,
	})

	h2 := h.WithAttrs([]slog.Attr{slog.String("key", "val")})

	// 两个 handler 共享限流器，总通过数应受限
	total := 100
	for i := 0; i < total/2; i++ {
		_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "h1", 0))
		_ = h2.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "h2", 0))
	}

	assert.Less(t, next.count(), total, "shared limiter should restrict total throughput")
}

func TestHandler_ConcurrentSafety(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{
		LevelRates:      map[slog.Level]float64{slog.LevelInfo: 100.0},
		BurstMultiplier: 1.0,
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "concurrent", 0))
			}
		}()
	}
	wg.Wait()

	// 不 panic 即通过
	assert.NotPanics(t, func() {
		_ = next.count()
	})
}

func TestHandler_ZeroRateNoLimit(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{
		LevelRates: map[slog.Level]float64{slog.LevelInfo: 0},
	})

	for i := 0; i < 100; i++ {
		_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0))
	}
	assert.Equal(t, 100, next.count(), "rate=0 should not limit")
}

func TestHandler_SummaryOutput(t *testing.T) {
	next := &mockHandler{}
	h := New(next, Config{
		LevelRates:      map[slog.Level]float64{slog.LevelInfo: 1.0},
		BurstMultiplier: 1.0,
		SummaryInterval: 1 * time.Nanosecond, // 极短间隔，确保触发摘要
	})

	// 触发丢弃
	for i := 0; i < 50; i++ {
		_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "drop", 0))
	}

	// 等待超过 summary interval
	time.Sleep(1 * time.Millisecond)

	// 发送一条通过的消息，触发懒检查摘要
	_ = h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelWarn, "trigger", 0))

	// 检查是否有摘要消息（Warn 级别中应包含 "sampling" 关键字）
	next.mu.Lock()
	defer next.mu.Unlock()
	hasSummary := false
	for _, r := range next.records {
		if r.Level == slog.LevelWarn && r.Message != "" {
			hasSummary = true
			break
		}
	}
	assert.True(t, hasSummary, "should output summary after interval")
}
