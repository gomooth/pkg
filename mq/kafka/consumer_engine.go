package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/kafka/internal"
	"github.com/gomooth/xerror"
)

const (
	ceIdle         int32 = 0
	ceRunning      int32 = 1
	ceShuttingDown int32 = 2
	ceClosed       int32 = 3
)

const maxConsumeErrors = 50

// consumerRegistration 内部消费者注册信息
type consumerRegistration struct {
	group   string
	topics  []string
	handler *groupHandler
	cg      sarama.ConsumerGroup
}

// consumerEngine 消费者生命周期引擎（未导出）
type consumerEngine struct {
	brokers []string
	config  *consumerConfig

	registrations []consumerRegistration
	regMu         sync.Mutex

	state      atomic.Int32
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	logger  *slog.Logger
	metrics *internal.ConsumerMetrics
}

// 编译时接口检查
var _ IConsumeServer = (*consumerEngine)(nil)

func newConsumerEngine(brokers []string, cfg *consumerConfig) *consumerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	// 初始化 sarama 全局日志器（仅首次调用生效）
	internal.InitSaramaLogger(logger)

	saramaConfig := cfg.saramaConfig
	if saramaConfig == nil {
		timeout := cfg.timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		saramaConfig = internal.BuildConsumerConfig(timeout)
	}

	metrics := internal.NewConsumerMetrics()

	engine := &consumerEngine{
		brokers: brokers,
		config:  cfg,
		logger:  logger,
		metrics: metrics,
	}

	// 预注册配置中的消费者
	for _, reg := range cfg.consumers {
		if err := engine.createRegistration(reg.Group, reg.Handler, reg.Topics, saramaConfig); err != nil {
			logger.Error("failed to create consumer group registration", "group", reg.Group, "error", err)
		}
	}

	return engine
}

func (e *consumerEngine) Register(group string, handler IHandler, topic string, topics ...string) {
	allTopics := append([]string{topic}, topics...)
	e.regMu.Lock()
	defer e.regMu.Unlock()

	if e.state.Load() != ceIdle {
		e.logger.Error("cannot register after consumer started", "group", group)
		return
	}

	saramaConfig := e.config.saramaConfig
	if saramaConfig == nil {
		timeout := e.config.timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		saramaConfig = internal.BuildConsumerConfig(timeout)
	}

	if err := e.createRegistration(group, handler, allTopics, saramaConfig); err != nil {
		e.logger.Error("failed to create consumer group registration", "group", group, "error", err)
	}
}

func (e *consumerEngine) Count() uint {
	e.regMu.Lock()
	defer e.regMu.Unlock()
	return uint(len(e.registrations))
}

func (e *consumerEngine) createRegistration(group string, handler IHandler, topics []string, saramaConfig *sarama.Config) error {
	for _, t := range topics {
		if len(t) == 0 {
			return xerror.New("kafka: topic must not be empty")
		}
	}

	cg, err := sarama.NewConsumerGroup(e.brokers, group, saramaConfig)
	if err != nil {
		return xerror.Wrap(err, "create consumer group client failed")
	}

	// 查找消费组级别的失败处理器
	failedHandler := e.config.failedHandler
	if e.config.groupFailedHandlers != nil {
		if gh, ok := e.config.groupFailedHandlers[group]; ok {
			failedHandler = gh
		}
	}

	// 检查 handler 是否实现 DeadLetterHandler
	var deadLetter DeadLetterHandler
	if dl, ok := handler.(DeadLetterHandler); ok {
		deadLetter = dl
	}

	gh := newGroupHandler(group, &groupHandlerConf{
		Logger:                   e.logger,
		Handler:                  handler,
		MaxRetry:                 e.config.maxRetry,
		Backoff:                  e.config.backoff,
		FailedHandler:            failedHandler,
		DeadLetter:               deadLetter,
		RetryMode:                e.config.retryMode,
		RetryWorkers:             e.config.retryWorkers,
		RetryStore:               e.config.retryStore,
		Metrics:                  e.metrics,
		HandlerTimeout:           e.config.handlerTimeout,
		SyncRetryMaxTotalTimeout: e.config.syncRetryMaxTotalTimeout,
	})

	e.registrations = append(e.registrations, consumerRegistration{
		group:   group,
		topics:  topics,
		handler: gh,
		cg:      cg,
	})
	return nil
}

func (e *consumerEngine) Start(ctx context.Context) error {
	if !e.state.CompareAndSwap(ceIdle, ceRunning) {
		if e.state.Load() == ceRunning {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQConsume, "consumer already closed")
	}

	engineCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel

	e.regMu.Lock()
	regs := make([]consumerRegistration, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()

	if len(regs) == 0 {
		cancel()
		e.state.Store(ceIdle)
		return xerror.NewXCode(xcode.ErrMQConsume, "no consumers registered")
	}

	for _, reg := range regs {
		e.wg.Add(1)
		r := reg
		e.safeGo(fmt.Sprintf("consume-%s", r.group), func() {
			defer e.wg.Done()
			e.handle(engineCtx, r.cg, r.topics, r.handler)
		})
	}

	return nil
}

func (e *consumerEngine) handle(ctx context.Context, cg sarama.ConsumerGroup, topics []string, handler *groupHandler) {
	e.logger.Debug("topic consumer handle start", "topics", topics)
	backoff := &retry.ExponentialDelay{Base: time.Second, Max: 30 * time.Second, Jitter: true}
	attempt := uint(0)

	for {
		if e.state.Load() != ceRunning {
			return
		}

		if err := cg.Consume(ctx, topics, handler); err != nil {
			e.logger.Error("topic consume failed", "topics", topics, "error", err)
			delay := backoff.Delay(attempt)
			attempt++

			if attempt >= maxConsumeErrors {
				e.logger.Error("topic consume exceeded max consecutive errors, pausing",
					"topics", topics, "attempts", attempt)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Minute):
					attempt = 0
					continue
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		} else {
			attempt = 0
		}

		if ctx.Err() != nil {
			e.logger.Error("topic consume context error", "topics", topics, "error", ctx.Err())
			return
		}
	}
}

func (e *consumerEngine) Shutdown(ctx context.Context) error {
	if !e.state.CompareAndSwap(ceRunning, ceShuttingDown) {
		return nil
	}

	// 通知 handler 排空重试队列
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	e.regMu.Lock()
	regs := make([]consumerRegistration, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()

	for _, reg := range regs {
		reg.handler.Shutdown(shutdownCtx)
	}

	// 关闭消费者组
	var firstErr error
	for _, reg := range regs {
		if reg.cg != nil {
			if err := reg.cg.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	e.wg.Wait()
	e.state.Store(ceClosed)
	return firstErr
}

func (e *consumerEngine) HealthCheck(_ context.Context) error {
	state := e.state.Load()
	if state != ceRunning {
		return xerror.NewXCode(xcode.ErrMQConsume, fmt.Sprintf("consumer not running (state=%d)", state))
	}
	return nil
}

func (e *consumerEngine) safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.logger.Error("goroutine panic recovered",
					"name", name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				if e.config.panicHandler != nil {
					e.config.panicHandler(r)
				}
			}
		}()
		fn()
	}()
}
