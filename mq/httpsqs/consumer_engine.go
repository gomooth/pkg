package httpsqs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/internal/consume"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/gomooth/xerror"
)

const maxConsumeErrors = 50

// queueConsumer 单个队列消费者
type queueConsumer struct {
	queueName string
	handler   types.IHandler
	client    httpsqs.IClient
	strategy  retryStrategy
}

// failedHandlerWrapper 失败回调包装器，提供并发安全和优雅关闭能力
type failedHandlerWrapper struct {
	fn types.FailedHandlerFunc
	wg sync.WaitGroup
}

func newFailedHandlerWrapper(fn types.FailedHandlerFunc) *failedHandlerWrapper {
	return &failedHandlerWrapper{fn: fn}
}

func (w *failedHandlerWrapper) Handle(ctx context.Context, msg types.Message, err error) {
	if w.fn == nil {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.fn(ctx, msg, err)
	}()
}

func (w *failedHandlerWrapper) Shutdown() {
	w.wg.Wait()
}

// consumerEngine 消费者生命周期引擎（未导出）
type consumerEngine struct {
	engine.Base
	opt *consumerConfig

	registrations []queueConsumer
	regMu         sync.Mutex

	failedWrapper *failedHandlerWrapper
	queueWrappers []*failedHandlerWrapper // 队列级别 failedHandler 的包装器，用于优雅关闭
}

// 编译时接口检查
var _ types.IConsumeServer = (*consumerEngine)(nil)

func newConsumerEngine(cfg *consumerConfig) *consumerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	m := metrics.NewConsumerMetrics("httpsqs")

	// 创建失败处理器包装器
	var failedFn types.FailedHandlerFunc
	if cfg.failedHandler != nil {
		failedFn = cfg.failedHandler
	} else {
		failedFn = DefaultFailedHandlerFunc(logutil.NewSlogLogger(logger))
	}
	failedWrapper := newFailedHandlerWrapper(failedFn)

	eng := &consumerEngine{
		Base: engine.Base{
			Logger:       logger,
			Metrics:      m,
			PanicHandler: cfg.panicHandler,
		},
		opt:           cfg,
		failedWrapper: failedWrapper,
	}

	// 预注册配置中的消费者
	for _, reg := range cfg.consumers {
		eng.createRegistration(reg.Queue, reg.Handler, reg.Opts)
	}

	return eng
}

func (e *consumerEngine) Register(queue string, handler types.IHandler, opts ...types.RegisterOption) error {
	e.regMu.Lock()
	defer e.regMu.Unlock()

	if e.State.Load() != engine.Idle {
		return xerror.NewXCode(xcode.ErrMQConsume, "cannot register after consumer started")
	}

	// 解析选项，HTTPSQS 不支持 WithGroup
	cfg := types.ApplyRegisterOptions(opts)
	if cfg.Group != "" {
		return xerror.NewXCode(xcode.ErrMQConsume, "httpsqs does not support WithGroup option")
	}

	e.createRegistration(queue, handler, cfg.QueueOpts)
	return nil
}

func (e *consumerEngine) Count() uint {
	e.regMu.Lock()
	defer e.regMu.Unlock()
	return uint(len(e.registrations))
}

func (e *consumerEngine) createRegistration(queueName string, handler types.IHandler, opts []types.QueueOption) {
	if len(queueName) == 0 {
		e.Logger.Error("queue name must not be empty")
		return
	}
	if handler == nil {
		e.Logger.Error("handler must not be nil", "queue", queueName)
		return
	}

	// 解析队列级别配置
	qCfg := &types.QueueConfig{}
	for _, opt := range opts {
		opt(qCfg)
	}

	// 确定使用的客户端：队列级别 > 全局
	var client httpsqs.IClient = e.opt.client
	if qCfg.Client != nil {
		// QueueConfig.Client 是 any 类型，需要断言为 httpsqs.IClient
		if ic, ok := qCfg.Client.(httpsqs.IClient); ok {
			client = ic
		}
	}

	// 确定重试参数
	maxRetry := e.opt.maxRetry
	if qCfg.MaxRetry != nil {
		maxRetry = *qCfg.MaxRetry
	}

	backoff := e.opt.backoff
	if qCfg.Backoff != nil {
		// QueueConfig.Backoff 是 any 类型，需要断言为 retry.BackoffStrategy
		if bs, ok := qCfg.Backoff.(retry.BackoffStrategy); ok {
			backoff = bs
		}
	}

	retryMode := e.opt.retryMode
	if qCfg.RetryMode != nil {
		retryMode = *qCfg.RetryMode
	}

	// 确定失败处理器：队列级别 > 全局（均通过 wrapper 异步调度，支持优雅关闭）
	var effectiveFailedFn types.FailedHandlerFunc
	if qCfg.FailedFn != nil {
		// 队列级别覆盖：创建独立的 wrapper 以支持优雅关闭
		qw := newFailedHandlerWrapper(qCfg.FailedFn)
		e.queueWrappers = append(e.queueWrappers, qw)
		effectiveFailedFn = func(ctx context.Context, msg types.Message, err error) {
			qw.Handle(ctx, msg, err)
		}
	} else {
		// 使用全局默认 wrapper
		effectiveFailedFn = func(ctx context.Context, msg types.Message, err error) {
			e.failedWrapper.Handle(ctx, msg, err)
		}
	}

	if backoff == nil {
		backoff = &retry.ExponentialDelay{Base: time.Second, Max: 5 * time.Minute}
	}

	intLogger := logutil.NewSlogLogger(e.Logger)
	m := e.Metrics.(*metrics.ConsumerMetrics)

	// 创建重试策略
	var strategy retryStrategy

	switch retryMode {
	case types.RetryModeRequeue:
		s := newRequeueRetryStrategy(handler, maxRetry, backoff, client, queueName, intLogger, m)
		s.SetFailedHandler(effectiveFailedFn)
		if dl, ok := handler.(types.DeadLetterHandler); ok {
			s.SetDeadLetterHandler(dl)
		}
		s.SetTimeout(e.opt.handlerTimeout)
		strategy = s
	default: // RetryModeSync
		s := newSyncRetryStrategy(handler, maxRetry, backoff, intLogger, m)
		s.SetFailedHandler(effectiveFailedFn)
		if dl, ok := handler.(types.DeadLetterHandler); ok {
			s.SetDeadLetterHandler(dl)
		}
		s.SetTimeout(e.opt.handlerTimeout)
		strategy = s
	}

	e.registrations = append(e.registrations, queueConsumer{
		queueName: queueName,
		handler:   handler,
		client:    client,
		strategy:  strategy,
	})
}

func (e *consumerEngine) Start(ctx context.Context) error {
	if !e.TryStart() {
		if e.State.Load() == engine.Running {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQConsume, "consumer already closed")
	}

	// 校验必填参数
	if e.opt.client == nil {
		e.State.Store(engine.Idle)
		return xerror.NewXCode(xcode.ErrMQConsume, "httpsqs client is required, use WithHTTPSQSClient()")
	}

	engineCtx, cancel := context.WithCancel(ctx)
	e.CancelFunc = cancel

	e.regMu.Lock()
	regs := make([]queueConsumer, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()

	if len(regs) == 0 {
		cancel()
		e.State.Store(engine.Idle)
		return xerror.NewXCode(xcode.ErrMQConsume, "no consumers registered")
	}

	// 启动消费循环
	for _, reg := range regs {
		r := reg
		e.WG.Add(1)
		e.SafeGo(fmt.Sprintf("consume-%s", r.queueName), func() {
			defer e.WG.Done()
			e.consumeLoop(engineCtx, r)
		}, e.opt.panicHandler)
	}

	return nil
}

func (e *consumerEngine) Shutdown(ctx context.Context) error {
	if !e.RequestShutdown() {
		if e.State.Load() == engine.Idle {
			e.State.Store(engine.Closed)
		}
		return nil
	}

	// 1. 取消 context，停止消费循环
	if e.CancelFunc != nil {
		e.CancelFunc()
	}

	// 2. 等待进行中的失败回调完成（消费循环已停止，不会再有新的调度）
	e.failedWrapper.Shutdown()
	for _, qw := range e.queueWrappers {
		qw.Shutdown()
	}

	// 3. 关闭 requeueRetryStrategy 的 AttemptTracker
	e.regMu.Lock()
	regs := make([]queueConsumer, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()
	for _, reg := range regs {
		if closer, ok := reg.strategy.(interface{ Close() }); ok {
			closer.Close()
		}
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
	return nil
}

// consumeLoop 单个队列的消费循环
func (e *consumerEngine) consumeLoop(ctx context.Context, qc queueConsumer) {
	fetcher := newHttpsqsFetcher(qc.client, qc.queueName)

	cfg := consume.LoopConfig{
		MQSystem:   "httpsqs",
		QueueName:  qc.queueName,
		EmptySleep: e.opt.emptyQueueSleep,
		MaxErrors:  maxConsumeErrors,
		Tracer:     telemetry.Tracer("mq.httpsqs.consumer"),
	}

	consume.ConsumeLoop(ctx, cfg, fetcher, qc.strategy)
}