package retry

import (
	"context"
	"time"

	"github.com/gomooth/pkg/mq/internal/attempt_tracker"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
)

// RequeueConfig 再入队重试策略配置
type RequeueConfig struct {
	MaxRetry      int
	Backoff       BackoffDelayFunc
	Tracker       *attempt_tracker.AttemptTracker
	Metrics       *metrics.ConsumerMetrics
	FailedHandler types.FailedHandlerFunc
	DeadLetter    types.DeadLetterHandler
	Timeout       time.Duration
	Requeue       func(ctx context.Context, msg types.Message) error
}

// RequeueStrategy 再入队重试策略
type RequeueStrategy struct {
	cfg RequeueConfig
}

// NewRequeueStrategy 创建再入队重试策略
func NewRequeueStrategy(cfg RequeueConfig) *RequeueStrategy {
	return &RequeueStrategy{cfg: cfg}
}

// SetFailedHandler 设置失败处理回调
func (s *RequeueStrategy) SetFailedHandler(fn types.FailedHandlerFunc) {
	s.cfg.FailedHandler = fn
}

// SetDeadLetterHandler 设置死信处理器
func (s *RequeueStrategy) SetDeadLetterHandler(h types.DeadLetterHandler) {
	s.cfg.DeadLetter = h
}

// SetTimeout 设置单次处理超时
func (s *RequeueStrategy) SetTimeout(d time.Duration) {
	s.cfg.Timeout = d
}

// OnMessage 对消息执行再入队重试策略。
// 失败时通过 Tracker 跟踪重试次数，未达上限则重新入队；
// 达到上限后调用 HandleExhausted。
func (s *RequeueStrategy) OnMessage(
	ctx context.Context,
	msg types.Message,
	handle func(ctx context.Context, msg types.Message) error,
) error {
	err := ApplyTimeout(ctx, s.cfg.Timeout, func(ctx context.Context) error {
		return handle(ctx, msg)
	})
	if err == nil {
		if s.cfg.Tracker != nil {
			key := attempt_tracker.MessageKey(string(msg.Data))
			s.cfg.Tracker.Remove(key)
		}
		if s.cfg.Metrics != nil {
			s.cfg.Metrics.OnConsume()
		}
		return nil
	}

	if s.cfg.Tracker == nil {
		HandleExhausted(ctx, s.cfg.Metrics, s.cfg.DeadLetter, s.cfg.FailedHandler, msg, err)
		return nil
	}

	key := attempt_tracker.MessageKey(string(msg.Data))
	attempt := s.cfg.Tracker.Increment(key)

	if attempt < s.cfg.MaxRetry {
		if s.cfg.Backoff != nil {
			delay := s.cfg.Backoff(uint(attempt - 1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if requeueErr := s.cfg.Requeue(ctx, msg); requeueErr != nil {
			s.cfg.Tracker.Remove(key)
			HandleExhausted(ctx, s.cfg.Metrics, s.cfg.DeadLetter, s.cfg.FailedHandler, msg, err)
			return nil
		}
		if s.cfg.Metrics != nil {
			s.cfg.Metrics.OnRetry()
		}
		return nil
	}

	s.cfg.Tracker.Remove(key)
	HandleExhausted(ctx, s.cfg.Metrics, s.cfg.DeadLetter, s.cfg.FailedHandler, msg, err)
	return nil
}

// Close 释放资源，停止 Tracker 后台清理 goroutine
func (s *RequeueStrategy) Close() {
	if s.cfg.Tracker != nil {
		s.cfg.Tracker.Close()
	}
}
