package kafka

import (
	"context"
	"fmt"
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
	engine.Base
	brokers []string
	config  *consumerConfig

	registrations []consumerRegistration
	regMu         sync.Mutex
}

// 编译时接口检查
var _ types.IConsumeServer = (*consumerEngine)(nil)

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

	m := metrics.NewConsumerMetrics("kafka")

	eng := &consumerEngine{
		Base: engine.Base{
			Logger:       logger,
			Metrics:      m,
			PanicHandler: cfg.panicHandler,
		},
		brokers: brokers,
		config:  cfg,
	}

	// 预注册配置中的消费者
	for _, reg := range cfg.consumers {
		if err := eng.createRegistration(reg.Group, reg.Handler, reg.Topics, saramaConfig); err != nil {
			logger.Error("failed to create consumer group registration", "group", reg.Group, "error", err)
		}
	}

	return eng
}

func (e *consumerEngine) Register(dest string, handler types.IHandler, opts ...types.RegisterOption) error {
	e.regMu.Lock()
	defer e.regMu.Unlock()

	if e.State.Load() != engine.Idle {
		return xerror.NewXCode(xcode.ErrMQConsume, "cannot register after consumer started")
	}

	// 解析选项，kafka 必须提供 WithGroup
	cfg := types.ApplyRegisterOptions(opts)
	if cfg.Group == "" {
		return xerror.NewXCode(xcode.ErrMQConsume, "kafka requires WithGroup option")
	}

	allTopics := append([]string{dest}, cfg.ExtraTopics...)

	saramaConfig := e.config.saramaConfig
	if saramaConfig == nil {
		timeout := e.config.timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		saramaConfig = internal.BuildConsumerConfig(timeout)
	}

	if err := e.createRegistration(cfg.Group, handler, allTopics, saramaConfig); err != nil {
		return err
	}
	return nil
}

func (e *consumerEngine) Count() uint {
	e.regMu.Lock()
	defer e.regMu.Unlock()
	return uint(len(e.registrations))
}

func (e *consumerEngine) createRegistration(group string, handler types.IHandler, topics []string, saramaConfig *sarama.Config) error {
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
	var deadLetter types.DeadLetterHandler
	if dl, ok := handler.(types.DeadLetterHandler); ok {
		deadLetter = dl
	}

	gh := newGroupHandler(group, &groupHandlerConf{
		Logger:                   e.Logger,
		Handler:                  handler,
		MaxRetry:                 e.config.maxRetry,
		Backoff:                  e.config.backoff,
		FailedHandler:            failedHandler,
		DeadLetter:               deadLetter,
		RetryMode:                e.config.retryMode,
		RetryWorkers:             e.config.retryWorkers,
		RetryStore:               e.config.retryStore,
		Metrics:                  e.Metrics,
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
	if !e.TryStart() {
		if e.State.Load() == engine.Running {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQConsume, "consumer already closed")
	}

	engineCtx, cancel := context.WithCancel(ctx)
	e.CancelFunc = cancel

	e.regMu.Lock()
	regs := make([]consumerRegistration, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()

	if len(regs) == 0 {
		cancel()
		e.State.Store(engine.Idle)
		return xerror.NewXCode(xcode.ErrMQConsume, "no consumers registered")
	}

	for _, reg := range regs {
		e.WG.Add(1)
		r := reg
		e.SafeGo(fmt.Sprintf("consume-%s", r.group), func() {
			defer e.WG.Done()
			e.handle(engineCtx, r.cg, r.topics, r.handler)
		}, e.config.panicHandler)
	}

	return nil
}

func (e *consumerEngine) handle(ctx context.Context, cg sarama.ConsumerGroup, topics []string, handler *groupHandler) {
	e.Logger.Debug("topic consumer handle start", "topics", topics)
	backoff := &retry.ExponentialDelay{Base: time.Second, Max: 30 * time.Second, Jitter: true}
	attempt := uint(0)

	for {
		if e.State.Load() != engine.Running {
			return
		}

		if err := cg.Consume(ctx, topics, handler); err != nil {
			e.Logger.Error("topic consume failed", "topics", topics, "error", err)
			delay := backoff.Delay(attempt)
			attempt++

			if attempt >= maxConsumeErrors {
				e.Logger.Error("topic consume exceeded max consecutive errors, pausing",
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
			e.Logger.Error("topic consume context error", "topics", topics, "error", ctx.Err())
			return
		}
	}
}

func (e *consumerEngine) Shutdown(ctx context.Context) error {
	if !e.RequestShutdown() {
		if e.State.Load() == engine.Idle {
			e.State.Store(engine.Closed)
		}
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

	e.State.Store(engine.Closed)
	return firstErr
}