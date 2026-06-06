package retry

import (
	"context"
	"time"

	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
)

// BackoffDelayFunc 退避延迟函数类型
type BackoffDelayFunc func(attempt uint) time.Duration

// SyncConfig 同步重试策略配置
type SyncConfig struct {
	MaxRetry      int
	Backoff       BackoffDelayFunc
	Metrics       *metrics.ConsumerMetrics
	FailedHandler types.FailedHandlerFunc
	DeadLetter    types.DeadLetterHandler
	Timeout       time.Duration
}

// SyncStrategy 同步阻塞重试策略
type SyncStrategy struct {
	cfg SyncConfig
}

// NewSyncStrategy 创建同步重试策略
func NewSyncStrategy(cfg SyncConfig) *SyncStrategy {
	return &SyncStrategy{cfg: cfg}
}

// SetFailedHandler 设置失败处理回调
func (s *SyncStrategy) SetFailedHandler(fn types.FailedHandlerFunc) {
	s.cfg.FailedHandler = fn
}

// SetDeadLetterHandler 设置死信处理器
func (s *SyncStrategy) SetDeadLetterHandler(h types.DeadLetterHandler) {
	s.cfg.DeadLetter = h
}

// SetTimeout 设置单次处理超时
func (s *SyncStrategy) SetTimeout(d time.Duration) {
	s.cfg.Timeout = d
}

// OnMessage 对消息执行同步重试策略。
// 首次执行 + 最多 MaxRetry 次重试；全部失败后调用 HandleExhausted。
func (s *SyncStrategy) OnMessage(
	ctx context.Context,
	msg types.Message,
	handle func(ctx context.Context, msg types.Message) error,
) error {
	var lastErr error
	for attempt := 0; attempt <= s.cfg.MaxRetry; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt > 0 && s.cfg.Backoff != nil {
			delay := s.cfg.Backoff(uint(attempt))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		err := ApplyTimeout(ctx, s.cfg.Timeout, func(ctx context.Context) error {
			return handle(ctx, msg)
		})
		if err == nil {
			if s.cfg.Metrics != nil {
				s.cfg.Metrics.OnConsume()
			}
			return nil
		}
		lastErr = err
		if attempt < s.cfg.MaxRetry {
			if s.cfg.Metrics != nil {
				s.cfg.Metrics.OnRetry()
			}
		}
	}
	HandleExhausted(ctx, s.cfg.Metrics, s.cfg.DeadLetter, s.cfg.FailedHandler, msg, lastErr)
	return lastErr
}
