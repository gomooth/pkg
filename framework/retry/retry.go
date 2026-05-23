package retry

import (
	"context"
	"math/rand/v2"
	"time"
)

// BackoffStrategy 退避策略接口，定义重试之间的延迟计算方式
type BackoffStrategy interface {
	Delay(attempt uint) time.Duration
}

// FixedDelay 固定延迟策略，每次重试之间等待相同的时间
type FixedDelay struct {
	Wait time.Duration
}

func (s *FixedDelay) Delay(_ uint) time.Duration { return s.Wait }

// LinearDelay 线性退避策略，延迟随重试次数线性增长：base * (attempt + 1)
type LinearDelay struct {
	Base time.Duration
}

func (s *LinearDelay) Delay(attempt uint) time.Duration {
	return s.Base * time.Duration(attempt+1)
}

// ExponentialDelay 指数退避策略，延迟随重试次数指数增长：base * 2^attempt
// 通过 Max 字段限制最大延迟；启用 Jitter 可防止多实例同时重试造成惊群效应
type ExponentialDelay struct {
	Base   time.Duration
	Max    time.Duration
	Jitter bool
}

func (s *ExponentialDelay) Delay(attempt uint) time.Duration {
	shift := attempt
	if shift > 30 {
		shift = 30
	}
	d := s.Base * time.Duration(1<<shift)
	if d > s.Max {
		d = s.Max
	}

	if s.Jitter && d > 0 {
		half := d / 2
		// Equal Jitter: delay/2 + random(0, delay/2)
		// 使用 math/rand/v2 全局源，Go 1.22+ 已无锁争用
		d = half + time.Duration(rand.Int64N(int64(half)))
	}
	return d
}

// defaultMaxAttempts MaxAttempts 为 0 时的默认最大尝试次数
const defaultMaxAttempts uint = 10

// InfiniteRetry 无限重试标志，设置 MaxAttempts = InfiniteRetry 表示无限重试
const InfiniteRetry uint = ^uint(0) // math.MaxUint

// Config 重试配置
type Config struct {
	// MaxAttempts 最大尝试次数（含首次调用），0 表示使用默认值 (10)
	// 设置为 -1（即 math.MaxUint）表示无限重试（不推荐，需确保 ctx 会取消）
	MaxAttempts uint
	// Strategy 退避策略，nil 时默认使用 FixedDelay{Wait: time.Second}
	Strategy BackoffStrategy
	// RetryIf 错误重试谓词，返回 true 时才重试；nil 时所有错误都重试
	RetryIf func(error) bool
}

// Do 执行带重试的操作。
// fn 的 attempt 参数从 0 开始（首次调用为 0）。
// 当 ctx 被取消或达到 MaxAttempts 时停止重试。
func Do(ctx context.Context, cfg Config, fn func(attempt uint) error) error {
	strategy := cfg.Strategy
	if strategy == nil {
		strategy = &FixedDelay{Wait: time.Second}
	}

	var lastErr error
	maxAttempts := cfg.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	for attempt := uint(0); ; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}

		if cfg.RetryIf != nil && !cfg.RetryIf(lastErr) {
			return lastErr
		}

		if maxAttempts > 0 && attempt+1 >= maxAttempts {
			return lastErr
		}

		delay := strategy.Delay(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}
