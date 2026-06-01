package kafka

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/kafka/internal"
	"github.com/gomooth/xerror"
	"go.opentelemetry.io/otel/codes"
)

const (
	producerIdle         int32 = 0
	producerRunning      int32 = 1
	producerShuttingDown int32 = 2
	producerClosed       int32 = 3
)

// producerEngine 生产者生命周期引擎（未导出）
type producerEngine struct {
	brokers []string
	timeout time.Duration
	logger  *slog.Logger

	mu     sync.RWMutex
	inner  sarama.SyncProducer
	config *sarama.Config

	state      atomic.Int32
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	reconnectCh chan struct{}
	metrics     *internal.ProducerMetrics
}

// 编译时接口检查
var _ IProducer = (*producerImpl)(nil)

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
		brokers:     brokers,
		timeout:     timeout,
		logger:      logger,
		config:      saramaConfig,
		reconnectCh: make(chan struct{}, 1),
		metrics:     internal.NewProducerMetrics(),
	}
}

func (e *producerEngine) Start(ctx context.Context) error {
	if !e.state.CompareAndSwap(producerIdle, producerRunning) {
		if e.state.Load() == producerRunning {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQPublish, "producer already closed")
	}

	// 初始连接
	p, err := sarama.NewSyncProducer(e.brokers, e.config)
	if err != nil {
		e.state.Store(producerIdle)
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	e.mu.Lock()
	e.inner = p
	e.mu.Unlock()

	engineCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel

	// 启动重连协程
	e.wg.Add(1)
	go e.reconnectLoop(engineCtx)

	return nil
}

func (e *producerEngine) Shutdown(ctx context.Context) error {
	if !e.state.CompareAndSwap(producerRunning, producerShuttingDown) {
		if e.state.Load() == producerIdle {
			e.state.Store(producerClosed)
		}
		return nil
	}

	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	e.markDisconnected()
	e.state.Store(producerClosed)
	return nil
}

func (e *producerEngine) Produce(ctx context.Context, topic string, message []byte) error {
	msgs := []*sarama.ProducerMessage{
		{Topic: topic, Value: sarama.ByteEncoder(message)},
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

func (e *producerEngine) ProduceBatch(ctx context.Context, topic string, messages ...[]byte) error {
	if len(messages) == 0 {
		return xerror.NewXCode(xcode.ErrMQPublish, "no messages")
	}
	msgs := make([]*sarama.ProducerMessage, len(messages))
	for i, msg := range messages {
		msgs[i] = &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(msg),
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

func (e *producerEngine) ProduceOrdered(ctx context.Context, topic string, partitionKey []byte, messages ...[]byte) error {
	if len(messages) == 0 {
		return xerror.NewXCode(xcode.ErrMQPublish, "no messages")
	}
	msgs := make([]*sarama.ProducerMessage, len(messages))
	for i, msg := range messages {
		msgs[i] = &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.ByteEncoder(partitionKey),
			Value: sarama.ByteEncoder(msg),
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
		if e.metrics != nil {
			e.metrics.OnError()
		}
		e.markDisconnected()
		e.triggerReconnect()
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	if e.metrics != nil {
		e.metrics.OnProduce(len(msgs))
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
	defer e.wg.Done()

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
					e.logger.Error("producer reconnect failed", "error", err, "attempt", attempt)
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

				e.logger.Info("producer reconnected successfully")
				attempt = 0
				break
			}
		}
	}
}

// healthCheck 健康检查（未导出，仅内部使用）
func (e *producerEngine) healthCheck(_ context.Context) error {
	if e.state.Load() != producerRunning {
		return xerror.NewXCode(xcode.ErrMQPublish, "producer not running")
	}
	return nil
}
