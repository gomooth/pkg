package redis

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/traceutil"
	"github.com/gomooth/pkg/mq/redis/internal"
	"github.com/gomooth/xerror"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	client    *redis.Client
	strategy  retryStrategy
}

// consumerEngine 消费者生命周期引擎（未导出）
type consumerEngine struct {
	addr string
	opt  *consumerConfig

	registrations []queueConsumer
	regMu         sync.Mutex

	state      atomic.Int32
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	logger  *slog.Logger
	metrics *metrics.ConsumerMetrics
}

// 编译时接口检查
var _ IConsumeServer = (*consumerEngine)(nil)

func newConsumerEngine(addr string, cfg *consumerConfig) *consumerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	metrics := metrics.NewConsumerMetrics("redis")

	engine := &consumerEngine{
		addr:    addr,
		opt:     cfg,
		logger:  logger,
		metrics: metrics,
	}

	// 预注册配置中的消费者
	for _, reg := range cfg.consumers {
		engine.createRegistration(reg.Queue, reg.Handler)
	}

	return engine
}

func (e *consumerEngine) Register(queue string, handler IHandler, _ ...string) {
	e.regMu.Lock()
	defer e.regMu.Unlock()

	if e.state.Load() != ceIdle {
		e.logger.Error("cannot register after consumer started", "queue", queue)
		return
	}

	e.createRegistration(queue, handler)
}

func (e *consumerEngine) Count() uint {
	e.regMu.Lock()
	defer e.regMu.Unlock()
	return uint(len(e.registrations))
}

func (e *consumerEngine) createRegistration(queueName string, handler IHandler) {
	if len(queueName) == 0 {
		e.logger.Error("queue name must not be empty")
		return
	}
	if handler == nil {
		e.logger.Error("handler must not be nil", "queue", queueName)
		return
	}

	// 创建 Redis 客户端
	opts := e.opt.redisOptions
	if opts == nil {
		opts = internal.BuildConsumerOptions(e.addr)
	}
	// 确保 Addr 正确
	opts.Addr = e.addr
	client := redis.NewClient(opts)

	// 创建重试策略
	var strategy retryStrategy
	backoff := e.opt.backoff
	if backoff == nil {
		backoff = &retry.ExponentialDelay{Base: time.Second, Max: 5 * time.Minute}
	}

	intLogger := logutil.NewSlogLogger(e.logger)

	// 注入默认 failedHandler（若用户未设置）
	failedHandler := e.opt.failedHandler
	if failedHandler == nil {
		failedHandler = DefaultFailedHandlerFunc(intLogger)
	}

	switch e.opt.retryMode {
	case RetryModeRequeue:
		s := newRequeueRetryStrategy(handler, e.opt.maxRetry, backoff, client, e.opt.queuePrefix, intLogger, e.metrics)
		s.SetFailedHandler(failedHandler)
		if dl, ok := handler.(DeadLetterHandler); ok {
			s.SetDeadLetterHandler(dl)
		}
		s.handlerTimeout = e.opt.handlerTimeout
		strategy = s
	default: // RetryModeSync
		s := newSyncRetryStrategy(handler, e.opt.maxRetry, backoff, intLogger, e.metrics)
		s.SetFailedHandler(failedHandler)
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

	// 验证连接
	for _, reg := range regs {
		if err := reg.client.Ping(ctx).Err(); err != nil {
			cancel()
			e.state.Store(ceIdle)
			return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
		}
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

	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// 关闭 Redis 客户端
	e.regMu.Lock()
	regs := make([]queueConsumer, len(e.registrations))
	copy(regs, e.registrations)
	e.regMu.Unlock()

	for _, reg := range regs {
		if reg.client != nil {
			_ = reg.client.Close()
		}
		// 关闭 requeueRetryStrategy 的 AttemptTracker
		if closer, ok := reg.strategy.(interface{ Close() }); ok {
			closer.Close()
		}
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
	queueKey := fmt.Sprintf("%s%s", e.opt.queuePrefix, qc.queueName)
	backupKey := fmt.Sprintf("%s_backup", queueKey)
	popTimeout := int64(5) // BLMOVE 阻塞 5 秒

	backoff := &retry.ExponentialDelay{Base: time.Second, Max: 30 * time.Second, Jitter: true}
	attempt := uint(0)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// BLMOVE 原子 Pop
		result, err := internal.PopScript.Run(ctx, qc.client, []string{queueKey, backupKey}, popTimeout).Text()
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			delay := backoff.Delay(attempt)
			attempt++
			e.logger.Error("pop from queue failed",
				"queue", qc.queueName,
				"attempt", attempt,
				"delay", delay,
				"error", err,
			)

			if attempt >= maxConsumeErrors {
				e.logger.Error("pop exceeded max consecutive errors, pausing",
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

		// 队列为空
		if len(result) == 0 {
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

		// Extract trace context
		msgCtx := traceutil.ExtractTraceContext(ctx, result)

		// Create consumer Span
		tracer := telemetry.Tracer("mq.redis.consumer")
		msgCtx, span := tracer.Start(msgCtx, fmt.Sprintf("%s consume", qc.queueName),
			trace.WithAttributes(
				attribute.String("messaging.system", "redis"),
				attribute.String("messaging.destination", qc.queueName),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)

		// 处理消息
		if err := qc.strategy.OnMessage(msgCtx, qc.queueName, []byte(result)); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if ctx.Err() != nil {
				span.End()
				return
			}
			e.logger.Error("strategy.OnMessage returned error",
				"queue", qc.queueName, "error", err)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
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
