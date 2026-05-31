package httpsqs

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomooth/httpsqs"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/httpsqs/internal"
	"github.com/gomooth/xerror"
)

const (
	ceIdle         int32 = 0
	ceRunning      int32 = 1
	ceShuttingDown int32 = 2
	ceClosed       int32 = 3
)

const maxConsumeErrors = 50

// queueConsumer 单个队列消费者
type queueConsumer struct {
	queueName string
	handler   IHandler
	client    httpsqs.IClient
	strategy  retryStrategy
}

// failedHandlerWrapper 失败回调包装器，提供并发安全和优雅关闭能力
type failedHandlerWrapper struct {
	fn FailedHandlerFunc
	wg sync.WaitGroup
}

func newFailedHandlerWrapper(fn FailedHandlerFunc) *failedHandlerWrapper {
	return &failedHandlerWrapper{fn: fn}
}

func (w *failedHandlerWrapper) Handle(ctx context.Context, queue string, data string, pos int64, err error) {
	if w.fn == nil {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.fn(ctx, queue, data, pos, err)
	}()
}

func (w *failedHandlerWrapper) Shutdown() {
	w.wg.Wait()
}

// consumerEngine 消费者生命周期引擎（未导出）
type consumerEngine struct {
	opt *consumerConfig

	registrations []queueConsumer
	regMu         sync.Mutex

	state      atomic.Int32
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	logger        *slog.Logger
	intLogger     internal.Logger
	metrics       *internal.ConsumerMetrics
	failedWrapper *failedHandlerWrapper
	queueWrappers []*failedHandlerWrapper // 队列级别 failedHandler 的包装器，用于优雅关闭
}

// 编译时接口检查
var _ IConsumeServer = (*consumerEngine)(nil)

func newConsumerEngine(cfg *consumerConfig) *consumerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	metrics := internal.NewConsumerMetrics()

	// 创建失败处理器包装器
	var failedFn FailedHandlerFunc
	if cfg.failedHandler != nil {
		failedFn = cfg.failedHandler
	} else {
		dh := newDefaultFailedHandler(logger)
		failedFn = func(ctx context.Context, queue string, data string, pos int64, err error) {
			dh.Print(ctx, queue, data, pos, err)
		}
	}
	failedWrapper := newFailedHandlerWrapper(failedFn)

	engine := &consumerEngine{
		opt:           cfg,
		logger:        logger,
		intLogger:     internal.NewSlogLogger(logger),
		metrics:       metrics,
		failedWrapper: failedWrapper,
	}

	// 预注册配置中的消费者
	for _, reg := range cfg.consumers {
		engine.createRegistration(reg.Queue, reg.Handler, reg.Opts)
	}

	return engine
}

func (e *consumerEngine) Register(queue string, handler IHandler, opts ...QueueOption) {
	e.regMu.Lock()
	defer e.regMu.Unlock()

	if e.state.Load() != ceIdle {
		e.logger.Error("cannot register after consumer started", "queue", queue)
		return
	}

	e.createRegistration(queue, handler, opts)
}

func (e *consumerEngine) Count() uint {
	e.regMu.Lock()
	defer e.regMu.Unlock()
	return uint(len(e.registrations))
}

func (e *consumerEngine) createRegistration(queueName string, handler IHandler, opts []QueueOption) {
	if len(queueName) == 0 {
		e.logger.Error("queue name must not be empty")
		return
	}
	if handler == nil {
		e.logger.Error("handler must not be nil", "queue", queueName)
		return
	}

	// 解析队列级别配置
	qCfg := &queueConfig{}
	for _, opt := range opts {
		opt(qCfg)
	}

	// 确定使用的客户端：队列级别 > 全局
	client := qCfg.client
	if client == nil {
		client = e.opt.client
	}

	// 确定重试参数
	maxRetry := e.opt.maxRetry
	if qCfg.maxRetry != nil {
		maxRetry = *qCfg.maxRetry
	}

	backoff := e.opt.backoff
	if qCfg.backoff != nil {
		backoff = qCfg.backoff
	}

	retryMode := e.opt.retryMode
	if qCfg.retryMode != nil {
		retryMode = *qCfg.retryMode
	}

	// 确定失败处理器：队列级别 > 全局（均通过 wrapper 异步调度，支持优雅关闭）
	var effectiveFailedFn FailedHandlerFunc
	if qCfg.failedFn != nil {
		// 队列级别覆盖：创建独立的 wrapper 以支持优雅关闭
		qw := newFailedHandlerWrapper(qCfg.failedFn)
		e.queueWrappers = append(e.queueWrappers, qw)
		effectiveFailedFn = func(ctx context.Context, queue string, data string, pos int64, err error) {
			qw.Handle(ctx, queue, data, pos, err)
		}
	} else {
		// 使用全局默认 wrapper
		effectiveFailedFn = func(ctx context.Context, queue string, data string, pos int64, err error) {
			e.failedWrapper.Handle(ctx, queue, data, pos, err)
		}
	}

	if backoff == nil {
		backoff = &retry.ExponentialDelay{Base: time.Second, Max: 5 * time.Minute}
	}

	// 创建重试策略
	var strategy retryStrategy

	switch retryMode {
	case RetryModeRequeue:
		s := newRequeueRetryStrategy(handler, maxRetry, backoff, client, queueName, e.intLogger, e.metrics)
		s.SetFailedHandler(effectiveFailedFn)
		if dl, ok := handler.(DeadLetterHandler); ok {
			s.SetDeadLetterHandler(dl)
		}
		s.handlerTimeout = e.opt.handlerTimeout
		strategy = s
	default: // RetryModeSync
		s := newSyncRetryStrategy(handler, maxRetry, backoff, e.intLogger, e.metrics)
		s.SetFailedHandler(effectiveFailedFn)
		if dl, ok := handler.(DeadLetterHandler); ok {
			s.SetDeadLetterHandler(dl)
		}
		s.handlerTimeout = e.opt.handlerTimeout
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
	if !e.state.CompareAndSwap(ceIdle, ceRunning) {
		if e.state.Load() == ceRunning {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQConsume, "consumer already closed")
	}

	// 校验必填参数
	if e.opt.client == nil {
		e.state.Store(ceIdle)
		return xerror.NewXCode(xcode.ErrMQConsume, "httpsqs client is required, use WithHTTPSQSClient()")
	}

	engineCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel

	e.regMu.Lock()
	regs := make([]queueConsumer, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()

	if len(regs) == 0 {
		cancel()
		e.state.Store(ceIdle)
		return xerror.NewXCode(xcode.ErrMQConsume, "no consumers registered")
	}

	// 启动消费循环
	for _, reg := range regs {
		r := reg
		e.wg.Add(1)
		e.safeGo(fmt.Sprintf("consume-%s", r.queueName), func() {
			defer e.wg.Done()
			e.consumeLoop(engineCtx, r)
		})
	}

	return nil
}

func (e *consumerEngine) Shutdown(ctx context.Context) error {
	if !e.state.CompareAndSwap(ceRunning, ceShuttingDown) {
		if e.state.Load() == ceIdle {
			e.state.Store(ceClosed)
		}
		return nil
	}

	// 1. 取消 context，停止消费循环
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// 2. 等待所有消费循环 goroutine 退出
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	// 3. 等待进行中的失败回调完成（消费循环已停止，不会再有新的调度）
	e.failedWrapper.Shutdown()
	for _, qw := range e.queueWrappers {
		qw.Shutdown()
	}

	e.state.Store(ceClosed)
	return nil
}

func (e *consumerEngine) HealthCheck(_ context.Context) error {
	state := e.state.Load()
	if state != ceRunning {
		return xerror.NewXCode(xcode.ErrMQConsume, fmt.Sprintf("consumer not running (state=%d)", state))
	}
	return nil
}

// consumeLoop 单个队列的消费循环
func (e *consumerEngine) consumeLoop(ctx context.Context, qc queueConsumer) {
	backoff := &retry.ExponentialDelay{Base: time.Second, Max: 30 * time.Second, Jitter: true}
	attempt := uint(0)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 从 HTTPSQS 拉取消息
		data, pos, err := qc.client.Get(ctx, qc.queueName)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			delay := backoff.Delay(attempt)
			attempt++
			e.logger.Error("fetch from httpsqs failed",
				"queue", qc.queueName,
				"attempt", attempt,
				"delay", delay,
				"error", err,
			)

			if attempt >= maxConsumeErrors {
				e.logger.Error("fetch exceeded max consecutive errors, pausing",
					"queue", qc.queueName, "attempts", attempt)
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
			continue
		}

		// 队列为空（pos == -1 是 HTTPSQS 约定）
		if pos == -1 {
			attempt = 0
			select {
			case <-ctx.Done():
				return
			case <-time.After(e.opt.emptyQueueSleep):
			}
			continue
		}

		// 成功获取消息，重置退避计数
		attempt = 0

		// 处理消息
		if err := qc.strategy.OnMessage(ctx, qc.queueName, data, pos); err != nil {
			if ctx.Err() != nil {
				return
			}
			e.logger.Error("strategy.OnMessage returned error",
				"queue", qc.queueName, "error", err)
		}
	}
}

// safeGo 启动带 panic 恢复的 goroutine
func (e *consumerEngine) safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.logger.Error("goroutine panic recovered",
					"name", name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				if e.opt.panicHandler != nil {
					e.opt.panicHandler(r)
				}
			}
		}()
		fn()
	}()
}
