package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/framework/metrics"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/xerror"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/errgroup"
)

var queueMeter = metrics.GetProvider().Meter("queue")

var (
	queueRestartCounter = queueMeter.Int64Counter("queue.restart.count")
	queueErrorCounter   = queueMeter.Int64Counter("queue.error.count")
)

// ConsumerError 记录 consumer 的异常信息
type ConsumerError struct {
	Consumer IConsumer
	Err      error
	Attempts uint // 已重试次数
}

// ServerOption server 配置选项
type ServerOption func(*server)

// WithMaxRestartPerConsumer 单个 consumer 最大重启次数，默认 3
func WithMaxRestartPerConsumer(n uint) ServerOption {
	return func(s *server) {
		s.maxRestart = n
	}
}

// WithOnConsumerError consumer 异常回调
func WithOnConsumerError(fn func(ConsumerError)) ServerOption {
	return func(s *server) {
		s.onError = fn
	}
}

// WithRestartBackoff 重启退避策略，默认使用 ExponentialDelay{Base: 5s, Max: 5min}
func WithRestartBackoff(strategy retry.BackoffStrategy) ServerOption {
	return func(s *server) {
		s.restartBackoff = strategy
	}
}

type server struct {
	consumers []IConsumer
	cancel    context.CancelFunc
	eg        *errgroup.Group

	maxRestart     uint
	onError        func(ConsumerError)
	restartBackoff retry.BackoffStrategy
}

var defaultRestartBackoff = &retry.ExponentialDelay{Base: 5 * time.Second, Max: 5 * time.Minute}

func NewServer(opts ...ServerOption) IConsumeServer {
	s := &server{
		consumers:      make([]IConsumer, 0),
		maxRestart:     3,
		restartBackoff: defaultRestartBackoff,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *server) Register(consumer IConsumer) {
	if consumer == nil {
		return
	}

	s.consumers = append(s.consumers, consumer)
}

func (s *server) Count() uint {
	return uint(len(s.consumers))
}

func (s *server) Start(ctx context.Context) error {
	if len(s.consumers) == 0 {
		return xerror.New("no register consumers")
	}

	ctx, s.cancel = context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	s.eg = eg

	for _, consumer := range s.consumers {
		c := consumer
		eg.Go(func() error {
			var attempts uint
			for {
				err := c.Consume(ctx)
				if ctx.Err() != nil {
					return nil // 正常关闭
				}

				attempts++

				// 记录重启指标
				queueRestartCounter.Add(ctx, 1, metric.WithAttributes(
					metrics.Attr("consumer", consumerName(c)),
				))

				if attempts >= s.maxRestart {
					// 记录错误指标（达到最大重启次数）
					queueErrorCounter.Add(ctx, 1, metric.WithAttributes(
						metrics.Attr("consumer", consumerName(c)),
					))
					s.notifyError(ConsumerError{Consumer: c, Err: err, Attempts: attempts})
					return nil // 放弃此 consumer，但不影响其他
				}

				s.notifyError(ConsumerError{Consumer: c, Err: err, Attempts: attempts})

				// 退避等待
				delay := s.restartBackoff.Delay(attempts - 1)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(delay):
				}
			}
		})
	}

	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	for _, c := range s.consumers {
		_ = c.Close()
	}
	if s.eg != nil {
		if err := s.eg.Wait(); err != nil && ctx.Err() == nil {
			return err
		}
	}
	return nil
}

// HealthCheck 实现 app.HealthChecker 接口
func (s *server) HealthCheck(_ context.Context) error {
	if len(s.consumers) == 0 {
		return xerror.New("queue server: no consumers registered")
	}
	return nil
}

func (s *server) notifyError(ce ConsumerError) {
	if s.onError != nil {
		s.onError(ce)
	}
}

// consumerName 获取 consumer 的类型名称，用于指标标签
func consumerName(c IConsumer) string {
	return fmt.Sprintf("%T", c)
}
