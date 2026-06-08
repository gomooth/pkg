package redis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/internal/consume"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/gomooth/pkg/mq/redis/internal"
	"github.com/gomooth/xerror"
	"github.com/redis/go-redis/v9"
)

const maxConsumeErrors = 50

// queueConsumer 单个队列消费者
type queueConsumer struct {
	queueName string
	handler   types.IHandler
	client    *redis.Client
	strategy  retryStrategy
}

// consumerEngine 消费者生命周期引擎（未导出）
type consumerEngine struct {
	engine.Base
	addr string
	opt  *consumerConfig

	registrations []queueConsumer
	regMu         sync.Mutex
}

// 编译时接口检查
var _ types.IConsumeServer = (*consumerEngine)(nil)

func newConsumerEngine(addr string, cfg *consumerConfig) *consumerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	m := metrics.NewConsumerMetrics("redis")

	eng := &consumerEngine{
		Base: engine.Base{
			Logger:       logger,
			Metrics:      m,
			PanicHandler: cfg.panicHandler,
		},
		addr: addr,
		opt:  cfg,
	}

	// 预注册配置中的消费者
	for _, reg := range cfg.consumers {
		eng.createRegistration(reg.Queue, reg.Handler)
	}

	return eng
}

func (e *consumerEngine) Register(queue string, handler types.IHandler, opts ...types.RegisterOption) error {
	e.regMu.Lock()
	defer e.regMu.Unlock()

	if e.State.Load() != engine.Idle {
		return xerror.NewXCode(xcode.ErrMQConsume, "cannot register after consumer started")
	}

	// 解析选项，Redis 不支持 WithGroup
	cfg := types.ApplyRegisterOptions(opts)
	if cfg.Group != "" {
		return xerror.NewXCode(xcode.ErrMQConsume, "redis does not support WithGroup option")
	}

	e.createRegistration(queue, handler)
	return nil
}

func (e *consumerEngine) Count() uint {
	e.regMu.Lock()
	defer e.regMu.Unlock()
	return uint(len(e.registrations))
}

func (e *consumerEngine) createRegistration(queueName string, handler types.IHandler) {
	if len(queueName) == 0 {
		e.Logger.Error("queue name must not be empty")
		return
	}
	if handler == nil {
		e.Logger.Error("handler must not be nil", "queue", queueName)
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

	intLogger := logutil.NewSlogLogger(e.Logger)
	m := e.Metrics.(*metrics.ConsumerMetrics)

	// 注入默认 failedHandler（若用户未设置）
	failedHandler := e.opt.failedHandler
	if failedHandler == nil {
		failedHandler = DefaultFailedHandlerFunc(intLogger)
	}

	switch e.opt.retryMode {
	case types.RetryModeRequeue:
		s := newRequeueRetryStrategy(handler, e.opt.maxRetry, backoff, client, e.opt.queuePrefix, intLogger, m)
		s.SetFailedHandler(failedHandler)
		if dl, ok := handler.(types.DeadLetterHandler); ok {
			s.SetDeadLetterHandler(dl)
		}
		s.SetTimeout(e.opt.handlerTimeout)
		strategy = s
	default: // RetryModeSync
		s := newSyncRetryStrategy(handler, e.opt.maxRetry, backoff, intLogger, m)
		s.SetFailedHandler(failedHandler)
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

	// 验证连接
	for _, reg := range regs {
		if err := reg.client.Ping(ctx).Err(); err != nil {
			cancel()
			e.State.Store(engine.Idle)
			return xerror.WrapWithXCode(err, xcode.ErrMQConsume)
		}
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

	if e.CancelFunc != nil {
		e.CancelFunc()
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
	queueKey := fmt.Sprintf("%s%s", e.opt.queuePrefix, qc.queueName)
	popTimeout := int64(5) // BLMOVE 阻塞 5 秒

	fetcher := newRedisFetcher(qc.client, queueKey, popTimeout)

	cfg := consume.LoopConfig{
		MQSystem:   "redis",
		QueueName:  qc.queueName,
		EmptySleep: e.opt.emptyQueueSleep,
		MaxErrors:  maxConsumeErrors,
		Tracer:     telemetry.Tracer("mq.redis.consumer"),
	}

	consume.ConsumeLoop(ctx, cfg, fetcher, qc.strategy)
}
