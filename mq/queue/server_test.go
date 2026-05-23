package queue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gomooth/pkg/framework/app"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/stretchr/testify/assert"
)

// mockConsumer implements IConsumer for testing
type mockConsumer struct {
	consumeFn func(ctx context.Context) error
	closeFn   func() error
}

func (m *mockConsumer) Consume(ctx context.Context) error {
	return m.consumeFn(ctx)
}

func (m *mockConsumer) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func TestServer_Register_NilSkipped(t *testing.T) {
	s := NewServer()
	s.Register(nil)
	s.Register(&mockConsumer{consumeFn: func(ctx context.Context) error { return nil }})
	s.Register(nil)

	assert.Equal(t, uint(1), s.Count())
}

func TestServer_Start_NoConsumers(t *testing.T) {
	s := NewServer()
	err := s.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no register consumers")
}

func TestServer_Shutdown_WaitsForGoroutines(t *testing.T) {
	var exited atomic.Int32

	s := NewServer()
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			<-ctx.Done()
			exited.Add(1)
			return nil
		},
	})

	err := s.Start(context.Background())
	assert.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), exited.Load())
}

func TestServer_ConsumerErrorIsolated(t *testing.T) {
	// 一个 consumer 失败不应影响其他 consumer
	var exited atomic.Int32

	s := NewServer(
		WithMaxRestartPerConsumer(1), // 快速放弃
	)
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			return errors.New("consumer failed")
		},
	})
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			<-ctx.Done()
			exited.Add(1)
			return nil
		},
	})

	err := s.Start(context.Background())
	assert.NoError(t, err)

	// 给 consumer 一些时间运行
	time.Sleep(100 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), exited.Load())
}

func TestServer_ConsumerAutoRestart(t *testing.T) {
	var runCount atomic.Int32

	s := NewServer(
		WithMaxRestartPerConsumer(3),
		WithRestartBackoff(&retry.FixedDelay{Wait: 10 * time.Millisecond}),
	)
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			runCount.Add(1)
			return errors.New("always fail")
		},
	})

	err := s.Start(context.Background())
	assert.NoError(t, err)

	// 等待 consumer 达到最大重启次数
	time.Sleep(500 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	// 初始运行 + 2次重启 = 3次（attempts=3 时达到 maxRestart 放弃）
	assert.Equal(t, int32(3), runCount.Load())
}

func TestServer_OnConsumerError(t *testing.T) {
	var errorCount atomic.Int32

	s := NewServer(
		WithMaxRestartPerConsumer(2),
		WithRestartBackoff(&retry.FixedDelay{Wait: 10 * time.Millisecond}),
		WithOnConsumerError(func(ce ConsumerError) {
			errorCount.Add(1)
		}),
	)
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			return errors.New("always fail")
		},
	})

	err := s.Start(context.Background())
	assert.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.True(t, errorCount.Load() >= 2)
}

func TestServer_MultipleConsumers(t *testing.T) {
	var exited atomic.Int32

	s := NewServer()
	for i := 0; i < 3; i++ {
		s.Register(&mockConsumer{
			consumeFn: func(ctx context.Context) error {
				<-ctx.Done()
				exited.Add(1)
				return nil
			},
		})
	}

	assert.Equal(t, uint(3), s.Count())

	err := s.Start(context.Background())
	assert.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), exited.Load())
}

func TestServer_ShutdownBeforeStart(t *testing.T) {
	s := NewServer()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestServer_Shutdown_CallsConsumerClose(t *testing.T) {
	var closeCount atomic.Int32

	s := NewServer()
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		closeFn: func() error {
			closeCount.Add(1)
			return nil
		},
	})
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		closeFn: func() error {
			closeCount.Add(1)
			return nil
		},
	})

	err := s.Start(context.Background())
	assert.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), closeCount.Load(), "Shutdown should call Close on all consumers")
}

func TestServer_ImplementsHealthChecker(t *testing.T) {
	s := NewServer()

	hc, ok := s.(app.HealthChecker)
	assert.True(t, ok, "server should implement app.HealthChecker")

	// 无消费者时健康检查应失败
	err := hc.HealthCheck(context.Background())
	assert.Error(t, err)

	// 有消费者时健康检查应通过
	s.Register(&mockConsumer{
		consumeFn: func(ctx context.Context) error { return nil },
	})
	err = hc.HealthCheck(context.Background())
	assert.NoError(t, err)
}
