package kafka

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
)

// 编译时接口检查
var _ retryStrategy = (*asyncRetryEngine)(nil)

const defaultMaxRetryQueueSize = 10000

// asyncRetryEngine 统一异步重试引擎
type asyncRetryEngine struct {
	// 配置
	consumerGroup  string
	handler        types.IHandler
	maxRetry       int
	backoff        retry.BackoffStrategy
	handlerTimeout time.Duration
	numWorkers     int

	// 存储
	store RetryStore // MemoryRetryStore 或 RedisRetryStore

	// 提交策略
	strategy CommitStrategy

	// 生命周期（替代 shutdown channel，修复 P1）
	state      atomic.Int32
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	// sarama session
	sessionMu sync.RWMutex
	session   sarama.ConsumerGroupSession

	// 依赖
	logger        logutil.Logger
	metrics       *metrics.ConsumerMetrics
	failedHandler types.FailedHandlerFunc
	deadLetter    types.DeadLetterHandler
}

const (
	engineIdle         int32 = 0
	engineRunning      int32 = 1
	engineShuttingDown int32 = 2
)

func newAsyncRetryEngine(
	cg string,
	handler types.IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	handlerTimeout time.Duration,
	numWorkers int,
	logger logutil.Logger,
	metrics *metrics.ConsumerMetrics,
) *asyncRetryEngine {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
		if numWorkers < 1 {
			numWorkers = 1
		}
	}
	return &asyncRetryEngine{
		consumerGroup:  cg,
		handler:        handler,
		maxRetry:       maxRetry,
		backoff:        backoff,
		handlerTimeout: handlerTimeout,
		numWorkers:     numWorkers,
		logger:         logger,
		metrics:        metrics,
	}
}

func newAsyncRetryEngineWithStore(
	cg string,
	handler types.IHandler,
	maxRetry int,
	backoff retry.BackoffStrategy,
	handlerTimeout time.Duration,
	numWorkers int,
	store RetryStore,
	logger logutil.Logger,
	metrics *metrics.ConsumerMetrics,
) *asyncRetryEngine {
	engine := newAsyncRetryEngine(cg, handler, maxRetry, backoff, handlerTimeout, numWorkers, logger, metrics)
	engine.store = store
	// 根据存储类型选择提交策略
	if wmStore, ok := store.(WatermarkStore); ok {
		engine.strategy = newWatermarkStrategy(wmStore, engine.logger)
	} else {
		engine.strategy = newDirectMarkStrategy(store, engine.logger)
	}
	return engine
}

// SetFailedHandler 设置失败处理器
func (e *asyncRetryEngine) SetFailedHandler(fn types.FailedHandlerFunc) {
	e.failedHandler = fn
}

// SetDeadLetterHandler 设置死信处理器
func (e *asyncRetryEngine) SetDeadLetterHandler(h types.DeadLetterHandler) {
	e.deadLetter = h
}

func (e *asyncRetryEngine) SetSession(session sarama.ConsumerGroupSession) {
	e.sessionMu.Lock()
	e.session = session
	e.sessionMu.Unlock()

	ctx, cancel := context.WithCancel(session.Context())
	e.cancelFunc = cancel
	e.state.Store(engineRunning)

	e.startWorkers(ctx)

	// 恢复 pending（仅 Redis 模式有 store 且需要恢复，memory 模式无持久化数据可恢复）
	if e.store != nil {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.recoverPending(ctx)
		}()
	}
}

func (e *asyncRetryEngine) ClearSession() {
	// 停止 worker（幂等，修复 P1）
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	e.wg.Wait()

	// 修复 P4：提交策略清理
	e.strategy.OnClearSession()

	e.sessionMu.Lock()
	e.session = nil
	e.sessionMu.Unlock()

	e.state.Store(engineIdle)
}

func (e *asyncRetryEngine) OnShutdown(shutdownCtx context.Context) {
	e.state.Store(engineShuttingDown)

	if e.cancelFunc != nil {
		e.cancelFunc() // 幂等！不会 panic（修复 P1）
	}

	// 通知提交策略的等待 goroutine
	e.strategy.OnShutdown(shutdownCtx)

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if e.logger != nil {
					e.logger.Error("wg.Wait goroutine panic recovered", "panic", r)
				}
			}
		}()
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-shutdownCtx.Done():
	}
}

func (e *asyncRetryEngine) OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	// 修复 P9：统一应用 handlerTimeout
	msgCtx, cancel := e.applyHandlerTimeout(ctx)
	defer cancel()

	kafkaMsg := types.NewKafkaMessage(e.consumerGroup, msg.Topic, msg.Value)
	err := e.handler.Handle(msgCtx, kafkaMsg)

	if err == nil {
		e.strategy.OnSuccess(ctx, session, &RetryItem{Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset})
		if e.metrics != nil {
			e.metrics.OnConsume()
		}
		return
	}

	// maxRetry == 0 表示不重试
	if e.maxRetry == 0 {
		result := handleExhausted(ctx, e.consumerGroup, msg.Topic, msg.Value, err,
			e.deadLetter, e.failedHandler, e.logger, e.metrics)
		if result == exhaustedHandled {
			e.strategy.OnExhausted(ctx, session, &RetryItem{Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset})
		}
		return
	}

	// 记录跟踪的 partition
	e.strategy.OnEnqueue(msg)

	if e.metrics != nil {
		e.metrics.OnRetry()
	}

	// 放入 RetryStore
	// 注意：MemoryRetryStore.Schedule 内部会调用 tracker.MarkPending，
	// 如果 pending 超限会返回 ErrRetryQueueFull
	nextRetryAt := time.Now().Add(e.backoff.Delay(0))
	item := &RetryItem{
		Topic:         msg.Topic,
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		Key:           msg.Key,
		Value:         msg.Value,
		Headers:       saramaHeadersToPublic(msg.Headers),
		Attempt:       1,
		NextRetryAt:   nextRetryAt,
		ConsumerGroup: e.consumerGroup,
	}

	if storeErr := e.store.Schedule(ctx, item); storeErr != nil {
		if e.logger != nil {
			e.logger.Error("schedule retry failed, degrading to exhausted handling",
				"topic", msg.Topic, "offset", msg.Offset, "error", storeErr)
		}
		// Schedule 失败意味着 pending 未被标记（或已满），无需 RemovePending
		result := handleExhausted(ctx, e.consumerGroup, msg.Topic, msg.Value, err,
			e.deadLetter, e.failedHandler, e.logger, e.metrics)
		if result == exhaustedHandled {
			e.strategy.OnScheduleFailed(ctx, session, &RetryItem{Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset})
		}
		return
	}

	// 持久化成功后通知策略
	e.strategy.MarkImmediate(session, msg)
}

// applyHandlerTimeout 统一包装 handler 超时（修复 P9）
func (e *asyncRetryEngine) applyHandlerTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if e.handlerTimeout > 0 {
		return context.WithTimeout(ctx, e.handlerTimeout)
	}
	return ctx, func() {}
}

// startWorkers 启动 worker 协程
func (e *asyncRetryEngine) startWorkers(ctx context.Context) {
	for i := 0; i < e.numWorkers; i++ {
		e.strategy.StartWorkers(ctx, &e.wg, e.processRetry)
	}
}

// processRetry 处理一次重试（修复 P9：统一应用 handlerTimeout）
func (e *asyncRetryEngine) processRetry(ctx context.Context, item *RetryItem) {
	msgCtx, cancel := e.applyHandlerTimeout(ctx)
	defer cancel()

	kafkaMsg := types.NewKafkaMessage(e.consumerGroup, item.Topic, item.Value)
	err := e.handler.Handle(msgCtx, kafkaMsg)

	if err == nil {
		// 重试成功
		e.strategy.OnSuccess(ctx, e.getSession(), item)
		if e.metrics != nil {
			e.metrics.OnConsume()
		}
		return
	}

	if item.Attempt < e.maxRetry {
		if e.metrics != nil {
			e.metrics.OnRetry()
		}
		newItem := &RetryItem{
			Topic:         item.Topic,
			Partition:     item.Partition,
			Offset:        item.Offset,
			Key:           item.Key,
			Value:         item.Value,
			Headers:       item.Headers,
			Attempt:       item.Attempt + 1,
			NextRetryAt:   time.Now().Add(e.backoff.Delay(uint(item.Attempt))),
			ConsumerGroup: item.ConsumerGroup,
		}

		if rescheduleErr := e.store.Reschedule(ctx, item, newItem); rescheduleErr != nil {
			if e.logger != nil {
				e.logger.Error("reschedule failed", "topic", item.Topic, "offset", item.Offset, "error", rescheduleErr)
			}
		}
		return
	}

	// 重试耗尽
	result := handleExhausted(ctx, e.consumerGroup, item.Topic, item.Value, err,
		e.deadLetter, e.failedHandler, e.logger, e.metrics)
	if result == exhaustedHandled {
		e.strategy.OnExhausted(ctx, e.getSession(), item)
	}
}

// recoverPending 启动时从 Redis 恢复待重试项
func (e *asyncRetryEngine) recoverPending(ctx context.Context) {
	if e.store == nil {
		return
	}
	items, err := e.store.LoadAll(ctx)
	if err != nil {
		if e.logger != nil {
			e.logger.Error("load pending retries failed", "error", err)
		}
		return
	}

	if len(items) == 0 {
		return
	}

	if e.logger != nil {
		e.logger.Info("recovered pending retries", "count", len(items))
	}

	now := time.Now()
	for _, item := range items {
		if item.NextRetryAt.Before(now) {
			item.NextRetryAt = now
		}
		if scheduleErr := e.store.Schedule(ctx, item); scheduleErr != nil {
			if e.logger != nil {
				e.logger.Error("reschedule recovered item failed",
					"topic", item.Topic, "offset", item.Offset, "error", scheduleErr)
			}
		}
	}
}

func (e *asyncRetryEngine) getSession() sarama.ConsumerGroupSession {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()
	return e.session
}

// saramaHeadersToPublic 转换 sarama 记录头为公开 HeaderKV
func saramaHeadersToPublic(headers []*sarama.RecordHeader) []HeaderKV {
	result := make([]HeaderKV, len(headers))
	for i, h := range headers {
		result[i] = HeaderKV{Key: string(h.Key), Value: h.Value}
	}
	return result
}