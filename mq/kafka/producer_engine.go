package kafka

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/gomooth/pkg/mq/kafka/internal"
	"github.com/gomooth/xerror"
	"go.opentelemetry.io/otel/codes"
)

// producerEngine 生产者生命周期引擎（未导出）
type producerEngine struct {
	engine.Base
	brokers []string
	timeout time.Duration

	mu     sync.RWMutex
	inner  sarama.SyncProducer
	config *sarama.Config

	reconnectCh chan struct{}
}

func newProducerEngine(brokers []string, cfg *producerConfig) *producerEngine {
	timeout := cfg.timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	// 初始化 sarama 全局日志器（仅首次调用生效）
	internal.InitSaramaLogger(logger)

	saramaConfig := cfg.saramaConfig
	if saramaConfig == nil {
		saramaConfig = internal.BuildProducerConfig(timeout)
	}

	return &producerEngine{
		Base: engine.Base{
			Logger:  logger,
			Metrics: metrics.NewProducerMetrics("kafka"),
		},
		brokers:     brokers,
		timeout:     timeout,
		config:      saramaConfig,
		reconnectCh: make(chan struct{}, 1),
	}
}

func (e *producerEngine) Start(ctx context.Context) error {
	if !e.TryStart() {
		if e.State.Load() == engine.Running {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQPublish, "producer already closed")
	}

	// 初始连接
	p, err := sarama.NewSyncProducer(e.brokers, e.config)
	if err != nil {
		e.State.Store(engine.Idle)
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	e.mu.Lock()
	e.inner = p
	e.mu.Unlock()

	engineCtx, cancel := context.WithCancel(ctx)
	e.CancelFunc = cancel

	// 启动重连协程
	e.WG.Add(1)
	go e.reconnectLoop(engineCtx)

	return nil
}

func (e *producerEngine) Shutdown(ctx context.Context) error {
	if !e.RequestShutdown() {
		if e.State.Load() == engine.Idle {
			e.State.Store(engine.Closed)
		}
		return nil
	}

	if e.CancelFunc != nil {
		e.CancelFunc()
	}

	done := make(chan struct{})
	go func() {
		e.WG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	e.markDisconnected()
	e.State.Store(engine.Closed)
	return nil
}

func (e *producerEngine) Produce(ctx context.Context, topic string, message []byte, opts ...types.ProduceOption) error {
	produceCfg := types.ApplyProduceOptions(opts)

	msgs := []*sarama.ProducerMessage{
		{Topic: topic, Value: sarama.ByteEncoder(message)},
	}

	// If OrderKey is set, use ordered production logic (merged from ProduceOrdered)
	if produceCfg.OrderKey != "" {
		msgs[0].Key = sarama.ByteEncoder(produceCfg.OrderKey)
	}

	ctx, span := injectProducerTrace(ctx, topic, msgs)
	defer span.End()

	err := e.send(ctx, msgs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (e *producerEngine) ProduceBatch(ctx context.Context, topic string, messages [][]byte, opts ...types.ProduceOption) error {
	if len(messages) == 0 {
		return xerror.NewXCode(xcode.ErrMQPublish, "no messages")
	}

	produceCfg := types.ApplyProduceOptions(opts)

	msgs := make([]*sarama.ProducerMessage, len(messages))
	for i, msg := range messages {
		msgs[i] = &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(msg),
		}
		// If OrderKey is set, apply to all messages in batch
		if produceCfg.OrderKey != "" {
			msgs[i].Key = sarama.ByteEncoder(produceCfg.OrderKey)
		}
	}

	ctx, span := injectProducerTrace(ctx, topic, msgs)
	defer span.End()

	err := e.send(ctx, msgs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (e *producerEngine) send(ctx context.Context, msgs []*sarama.ProducerMessage) error {
	if err := ctx.Err(); err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	e.mu.RLock()
	producer := e.inner
	e.mu.RUnlock()

	if producer == nil {
		return xerror.NewXCode(xcode.ErrMQPublish, "producer not connected")
	}

	err := producer.SendMessages(msgs)
	if err != nil {
		if m, ok := e.Metrics.(*metrics.ProducerMetrics); ok && m != nil {
			m.OnError()
		}
		e.markDisconnected()
		e.triggerReconnect()
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	if m, ok := e.Metrics.(*metrics.ProducerMetrics); ok && m != nil {
		m.OnProduce(len(msgs))
	}
	return nil
}

func (e *producerEngine) markDisconnected() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.inner != nil {
		_ = e.inner.Close()
		e.inner = nil
	}
}

func (e *producerEngine) triggerReconnect() {
	select {
	case e.reconnectCh <- struct{}{}:
	default:
	}
}

func (e *producerEngine) reconnectLoop(ctx context.Context) {
	defer e.WG.Done()

	backoffStrategy := &retry.ExponentialDelay{
		Base:   1 * time.Second,
		Max:    30 * time.Second,
		Jitter: true,
	}
	attempt := uint(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.reconnectCh:
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				p, err := sarama.NewSyncProducer(e.brokers, e.config)
				if err != nil {
					e.Logger.Error("producer reconnect failed", "error", err, "attempt", attempt)
					delay := backoffStrategy.Delay(attempt)
					attempt++
					select {
					case <-time.After(delay):
					case <-ctx.Done():
						return
					}
					continue
				}

				e.mu.Lock()
				e.inner = p
				e.mu.Unlock()

				e.Logger.Info("producer reconnected successfully")
				attempt = 0
				break
			}
		}
	}
}

// healthCheck 健康检查（未导出，仅内部使用）
func (e *producerEngine) healthCheck(_ context.Context) error {
	if e.State.Load() != engine.Running {
		return xerror.NewXCode(xcode.ErrMQPublish, "producer not running")
	}
	return nil
}