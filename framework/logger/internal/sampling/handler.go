package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// Config 采样配置
type Config struct {
	// LevelRates 按 slog.Level 配置每秒允许的日志条数。
	// 未配置的级别不限流，0 表示该级别不限。
	LevelRates map[slog.Level]float64

	// BurstMultiplier 突发容量倍数，burst = max(1, int(rate * BurstMultiplier))，默认 1.0
	BurstMultiplier float64

	// SummaryInterval 丢弃摘要间隔，0 表示不输出摘要
	SummaryInterval time.Duration
}

// Handler 基于令牌桶的采样 Handler，实现 slog.Handler 接口。
// 按日志级别独立限流，未配置的级别不限流。
type Handler struct {
	Next     slog.Handler
	limiters map[slog.Level]*rate.Limiter
	cfg      Config

	dropped map[slog.Level]*atomic.Int64
	since   *atomic.Int64 // unix nano of last summary output
}

// New 创建采样 Handler。
// 采样应在 trace.Injector 之前插入链路，避免对即将丢弃的日志做无用的 trace 注入。
func New(next slog.Handler, cfg Config) *Handler {
	bm := cfg.BurstMultiplier
	if bm <= 0 {
		bm = 1.0
	}

	limiters := make(map[slog.Level]*rate.Limiter, len(cfg.LevelRates))
	dropped := make(map[slog.Level]*atomic.Int64, len(cfg.LevelRates))
	for level, r := range cfg.LevelRates {
		if r <= 0 {
			continue
		}
		burst := max(1, int(r*bm))
		limiters[level] = rate.NewLimiter(rate.Limit(r), burst)
		dropped[level] = new(atomic.Int64)
	}

	return &Handler{
		Next:     next,
		limiters: limiters,
		cfg:      cfg,
		dropped:  dropped,
		since:    new(atomic.Int64),
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Next.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	limiter := h.limiters[r.Level]
	if limiter == nil {
		// 未配置限流的级别，直接放行
		h.maybeSummary(ctx, r.Level)
		return h.Next.Handle(ctx, r)
	}

	if limiter.Allow() {
		h.maybeSummary(ctx, r.Level)
		return h.Next.Handle(ctx, r)
	}

	// 丢弃，增加计数
	if counter := h.dropped[r.Level]; counter != nil {
		counter.Add(1)
	}

	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		Next:     h.Next.WithAttrs(attrs),
		limiters: h.limiters,
		cfg:      h.cfg,
		dropped:  h.dropped,
		since:    h.since,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		Next:     h.Next.WithGroup(name),
		limiters: h.limiters,
		cfg:      h.cfg,
		dropped:  h.dropped,
		since:    h.since,
	}
}

// maybeSummary 懒检查方式输出丢弃摘要：在日志成功通过时检查时间间隔，
// 如果距离上次摘要已超过 SummaryInterval，则输出摘要并重置计数。
func (h *Handler) maybeSummary(ctx context.Context, currentLevel slog.Level) {
	interval := h.cfg.SummaryInterval
	if interval <= 0 {
		return
	}

	now := time.Now().UnixNano()
	last := h.since.Load()

	if now-last < int64(interval) {
		return
	}

	// CAS 确保只有一个 handler 实例输出摘要
	if !h.since.CompareAndSwap(last, now) {
		return
	}

	for level, counter := range h.dropped {
		n := counter.Swap(0)
		if n > 0 {
			_ = h.Next.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelWarn, fmt.Sprintf("sampling: %d %s-level logs dropped since last summary", n, level.String()), 0))
		}
	}
}
