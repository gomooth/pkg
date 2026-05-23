package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomooth/pkg/framework/logger"
	"github.com/gomooth/pkg/framework/retry"

	"github.com/gomooth/xerror"

	"github.com/IBM/sarama"
)

// IHandler 消息处理器接口，与父包 IHandler 签名一致
// 因 Go 包循环限制无法直接引用父包类型，在此定义等价接口
type IHandler interface {
	Handle(ctx context.Context, topic string, msg []byte) error
}

// IConsumer Kafka 消费者接口，封装消费组的生命周期与配置注入
type IConsumer interface {
	// SetLogger 设置日志器
	SetLogger(l *slog.Logger)

	// SetPanicHandler 设置 panic 恢复回调，参数 r 为 recover() 捕获的原始值
	SetPanicHandler(panicHandler func(r any))

	// SetMaxRetry 设置消息处理最大重试次数（0 表示不重试）
	SetMaxRetry(maxRetry uint)

	// SetBackoff 设置重试退避策略，nil 时使用默认指数退避
	SetBackoff(backoff retry.BackoffStrategy)

	// SetFailedHandler 设置消费失败回调，在所有重试耗尽后调用。
	//   - cg: 消费组名称
	//   - topic: 消息所属 Kafka topic
	//   - msg: 消息原始内容
	//   - err: 最终失败原因
	SetFailedHandler(handler func(ctx context.Context, cg, topic string, msg []byte, err error))

	// SetRetryMode 设置重试模式配置
	//   - RetryModeSync: 同步阻塞重试，消息处理失败时在消费协程内重试，阻塞 partition
	//   - RetryModeAsyncWatermark: 异步重试 + 水位线 offset 跟踪，不依赖外部存储
	//   - RetryModeAsyncRedis: 异步重试 + Redis 持久化，失败消息持久化到 Redis 后立即提交 offset
	SetRetryMode(mode RetryMode)

	// SetRetryWorkers 设置异步重试模式下的并发工作协程数，仅对异步重试模式生效
	SetRetryWorkers(n int)

	// SetRetryMaxQueueSize 设置异步重试内存队列的最大容量，超限时降级为直接处理（走死信/失败回调）。
	// 仅对 RetryModeAsyncWatermark 生效，0 表示无限制。
	SetRetryMaxQueueSize(n int)

	// SetRetryRedisStore 设置异步 Redis 重试模式的存储后端，仅 RetryModeAsyncRedis 模式下需要
	SetRetryRedisStore(store RedisRetryStore)

	// SetSyncRetryMaxTotalTimeout 设置同步重试模式下单条消息的总重试超时上限，0 表示不限
	SetSyncRetryMaxTotalTimeout(d time.Duration)

	// SetHandlerTimeout 设置单次 handler 调用的超时时间，0 表示不限（默认）。
	// 超时后该次尝试视为失败，可进入重试。对三种重试模式统一生效。
	SetHandlerTimeout(d time.Duration)

	// SetMetricsCallbacks 设置 Metrics 回调注入，用于记录消费/重试/死信等指标
	SetMetricsCallbacks(m MetricsCallbacks)

	// RegisterHandler 注册消费组处理器。
	//   - group: Kafka 消费组名称，同组的消费者共享 partition 分配
	//   - topics: 订阅的 Kafka topic 列表，不能为空
	//   - handler: 消息处理器，必须实现 Handle 方法
	//   - dlHandler: 死信处理器，消息重试耗尽后调用；nil 表示不处理死信。
	//     返回 error 表示死信处理自身失败，参数含义：
	//       ctx: 上下文, topic: 原始 topic, msg: 原始消息内容, err: 最终失败原因
	RegisterHandler(group string, topics []string, handler IHandler, dlHandler func(ctx context.Context, topic string, msg []byte, err error) error) error

	// Run 启动所有已注册的消费组，阻塞直到所有消费协程结束或 Context 取消
	Run(ctx context.Context)

	// Close 优雅关闭消费者，等待所有消费协程退出
	Close() error

	// IsRunning 返回消费者是否正在运行
	IsRunning() bool
}

type defaultConsumer struct {
	addrs []string

	maxRetry      uint
	backoff       retry.BackoffStrategy
	panicHandler  func(any)
	logger        *slog.Logger
	failedHandler func(ctx context.Context, consumerGroup, topic string, msg []byte, err error)

	// 新增：重试模式配置
	retryMode         RetryMode
	retryWorkers      int
	retryMaxQueueSize int
	redisStore        RedisRetryStore

	// 新增：Metrics 回调
	metricsCallbacks MetricsCallbacks

	syncRetryMaxTotalTimeout time.Duration
	handlerTimeout           time.Duration

	handlers []*groupHandlerParam

	running uint32         // 是否运行中 (running: 1 stopped: 0)
	wg      sync.WaitGroup // 跟踪所有启动的 goroutine

	cg     sarama.ConsumerGroup
	config *sarama.Config
}

func NewDefaultConsumer(addrs []string) IConsumer {
	hostname, _ := os.Hostname()

	conf := sarama.NewConfig()
	conf.Version = sarama.V3_6_0_0
	conf.ClientID = hostname

	conf.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}

	// 默认启动一个日志收集（仅首次设置，避免多实例覆盖）
	l := logger.NewConsoleLogger()
	initSaramaLogger(l)

	return &defaultConsumer{
		config:   conf,
		addrs:    addrs,
		logger:   l,
		handlers: make([]*groupHandlerParam, 0),
	}
}

func (dcc *defaultConsumer) SetLogger(l *slog.Logger) {
	sarama.Logger = newSaramaLogger(l)
	dcc.logger = l
}

func (dcc *defaultConsumer) SetPanicHandler(panicHandler func(any)) {
	dcc.panicHandler = panicHandler
}

func (dcc *defaultConsumer) SetMaxRetry(maxRetry uint) {
	dcc.maxRetry = maxRetry
}

func (dcc *defaultConsumer) SetBackoff(backoff retry.BackoffStrategy) {
	dcc.backoff = backoff
}

func (dcc *defaultConsumer) SetFailedHandler(handler func(ctx context.Context, consumerGroup, topic string, msg []byte, err error)) {
	dcc.failedHandler = handler
}

func (dcc *defaultConsumer) SetRetryMode(mode RetryMode) {
	dcc.retryMode = mode
}

func (dcc *defaultConsumer) SetRetryWorkers(n int) {
	dcc.retryWorkers = n
}

func (dcc *defaultConsumer) SetRetryMaxQueueSize(n int) {
	dcc.retryMaxQueueSize = n
}

func (dcc *defaultConsumer) SetRetryRedisStore(store RedisRetryStore) {
	dcc.redisStore = store
}

func (dcc *defaultConsumer) SetSyncRetryMaxTotalTimeout(d time.Duration) {
	dcc.syncRetryMaxTotalTimeout = d
}

func (dcc *defaultConsumer) SetHandlerTimeout(d time.Duration) {
	dcc.handlerTimeout = d
}

func (dcc *defaultConsumer) SetMetricsCallbacks(m MetricsCallbacks) {
	dcc.metricsCallbacks = m
}

type groupHandlerParam struct {
	ConsumerGroup sarama.ConsumerGroup
	Topics        []string
	Handler       *groupHandler // 改为具体类型，以调用 Shutdown
}

func (dcc *defaultConsumer) RegisterHandler(group string, topics []string, handler IHandler, dlHandler func(context.Context, string, []byte, error) error) error {
	for _, s := range topics {
		if len(s) == 0 {
			return xerror.New("kafkaconsumer: topic must not be empty")
		}
	}

	cg, err := sarama.NewConsumerGroup(dcc.addrs, group, dcc.config)
	if err != nil {
		return xerror.Wrap(err, "create consumer group client failed")
	}

	gh := newConsumerGroupHandler(group, &groupHandlerConf{
		Logger:                   dcc.logger,
		Handler:                  handler.Handle,
		MaxRetry:                 dcc.maxRetry,
		Backoff:                  dcc.backoff,
		FailedHandler:            dcc.failedHandler,
		DeadLetterHandler:        dlHandler,
		RetryMode:                dcc.retryMode,
		RetryWorkers:             dcc.retryWorkers,
		RetryMaxQueueSize:        dcc.retryMaxQueueSize,
		RedisStore:               dcc.redisStore,
		Metrics:                  dcc.metricsCallbacks,
		SyncRetryMaxTotalTimeout: dcc.syncRetryMaxTotalTimeout,
		HandlerTimeout:           dcc.handlerTimeout,
	})
	dcc.handlers = append(dcc.handlers, &groupHandlerParam{
		ConsumerGroup: cg,
		Topics:        topics,
		Handler:       gh,
	})
	return nil
}

func (dcc *defaultConsumer) Run(ctx context.Context) {
	if atomic.LoadUint32(&dcc.running) == 1 {
		dcc.logger.Warn("consume could not called while running")
		return
	}

	atomic.StoreUint32(&dcc.running, 1)

	for _, handler := range dcc.handlers {
		dcc.wg.Add(1)
		dcc.safeGo(fmt.Sprintf("consume-%v", handler.Topics), func() {
			defer dcc.wg.Done()
			dcc.handle(ctx, handler.ConsumerGroup, handler.Topics, handler.Handler)
		})
	}
}

// maxConsumeErrors 连续消费错误上限，超过后进入长等待避免无限重连消耗资源
const maxConsumeErrors = 50

func (dcc *defaultConsumer) handle(ctx context.Context, cg sarama.ConsumerGroup, topics []string, handler sarama.ConsumerGroupHandler) {
	dcc.logger.Debug(fmt.Sprintf("topic consumer handle start, topics=%s", topics))
	backoff := &retry.ExponentialDelay{Base: time.Second, Max: 30 * time.Second, Jitter: true}
	attempt := uint(0)
	for {
		// 检查运行标志，Close() 后应退出
		if atomic.LoadUint32(&dcc.running) == 0 {
			return
		}

		if err := cg.Consume(ctx, topics, handler); err != nil {
			dcc.logger.Error(fmt.Sprintf("topic consume failed: [topics:%s] %s", topics, err))
			delay := backoff.Delay(attempt)
			attempt++

			// 连续失败超过上限，进入长等待，避免无限重连消耗资源
			if attempt >= maxConsumeErrors {
				dcc.logger.Error(fmt.Sprintf(
					"topic consume exceeded max consecutive errors, pausing: [topics:%s] attempts=%d",
					topics, attempt,
				))
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

		// check if context was cancelled, signaling that the consumer should stop
		if ctx.Err() != nil {
			dcc.logger.Error(fmt.Sprintf("topic consume context happen error, break: [topics:%s] %s", topics, ctx.Err()))
			return
		}
	}
}

func (dcc *defaultConsumer) IsRunning() bool {
	return atomic.LoadUint32(&dcc.running) == 1
}

func (dcc *defaultConsumer) Close() error {
	atomic.StoreUint32(&dcc.running, 0)

	// 通知所有重试策略优雅关闭（排空重试队列）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	for _, h := range dcc.handlers {
		h.Handler.Shutdown(shutdownCtx)
	}

	var firstErr error
	for _, h := range dcc.handlers {
		if h.ConsumerGroup != nil {
			if err := h.ConsumerGroup.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	dcc.wg.Wait()
	return firstErr
}

// safeGo 在 goroutine 中执行 fn，捕获 panic 并记录日志
func (dcc *defaultConsumer) safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				dcc.logger.Error("goroutine panic recovered",
					"name", name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				if dcc.panicHandler != nil {
					dcc.panicHandler(r)
				}
			}
		}()
		fn()
	}()
}
