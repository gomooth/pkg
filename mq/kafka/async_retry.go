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
)

// 编译时接口检查
var _ retryStrategy = (*asyncRetryEngine)(nil)

const defaultMaxRetryQueueSize = 10000

// topicPartition 用于 trackedParts map 的 key
type topicPartition struct {
	topic     string
	partition int32
}

// asyncRetryEngine 统一异步重试引擎
type asyncRetryEngine struct {
	// 配置
	consumerGroup  string
	handler        IHandler
	maxRetry       int
	backoff        retry.BackoffStrategy
	handlerTimeout time.Duration
	numWorkers     int

	// 存储
	store   RetryStore     // MemoryRetryStore 或 RedisRetryStore
	wmStore WatermarkStore // 非 nil 时为水位线模式

	// 当前跟踪的 partition 集合（用于 ClearSession 时 ResetPartition）
	trackedParts map[topicPartition]bool
	tpMu         sync.Mutex

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
	failedHandler FailedHandlerFunc
	deadLetter    DeadLetterHandler
}

const (
	engineIdle         int32 = 0
	engineRunning      int32 = 1
	engineShuttingDown int32 = 2
)

func newAsyncRetryEngine(
	cg string,
	handler IHandler,
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
		trackedParts:   make(map[topicPartition]bool),
		logger:         logger,
		metrics:        metrics,
	}
}

func newAsyncRetryEngineWithStore(
	cg string,
	handler IHandler,
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
	// 检查是否为水位线模式
	if wmStore, ok := store.(WatermarkStore); ok {
		engine.wmStore = wmStore
	}
	return engine
}

// SetFailedHandler 设置失败处理器
func (e *asyncRetryEngine) SetFailedHandler(fn FailedHandlerFunc) {
	e.failedHandler = fn
}

// SetDeadLetterHandler 设置死信处理器
func (e *asyncRetryEngine) SetDeadLetterHandler(h DeadLetterHandler) {
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

	// Redis 模式：恢复 pending
	if e.store != nil && e.wmStore == nil {
		e.wg.Add(1) // 修复 P7：recoverPending 加入 wg
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

	// 修复 P4：水位线模式下重置所有跟踪的 partition
	if e.wmStore != nil {
		e.tpMu.Lock()
		for tp := range e.trackedParts {
			e.wmStore.ResetPartition(tp.topic, tp.partition)
		}
		e.trackedParts = make(map[topicPartition]bool)
		e.tpMu.Unlock()
	}

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

	// 通知 wmStore 的等待 goroutine
	if e.wmStore != nil {
		select {
		case e.wmStore.Notify() <- struct{}{}:
		default:
		}
	}

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

	err := e.handler.Handle(msgCtx, msg.Topic, msg.Value)

	if err == nil {
		if e.wmStore != nil {
			e.wmStore.MarkSuccess(msg.Topic, msg.Partition, msg.Offset)
			e.commitWatermark(session, msg.Topic, msg.Partition)
		} else {
			session.MarkMessage(msg, "")
		}
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
			if e.wmStore != nil {
				e.wmStore.RemovePending(msg.Topic, msg.Partition, msg.Offset)
				e.commitWatermark(session, msg.Topic, msg.Partition)
			} else {
				session.MarkMessage(msg, "")
			}
		}
		return
	}

	// 记录跟踪的 partition
	e.tpMu.Lock()
	e.trackedParts[topicPartition{topic: msg.Topic, partition: msg.Partition}] = true
	e.tpMu.Unlock()

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
			if e.wmStore != nil {
				e.commitWatermark(session, msg.Topic, msg.Partition)
			} else {
				session.MarkMessage(msg, "")
			}
		}
		return
	}

	// Redis 模式：持久化成功即可提交 offset
	if e.wmStore == nil {
		session.MarkMessage(msg, "")
	}
}

// applyHandlerTimeout 统一包装 handler 超时（修复 P9）
func (e *asyncRetryEngine) applyHandlerTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if e.handlerTimeout > 0 {
		return context.WithTimeout(ctx, e.handlerTimeout)
	}
	return ctx, func() {}
}

// commitWatermark 提交水位线以内的 offset
func (e *asyncRetryEngine) commitWatermark(session sarama.ConsumerGroupSession, topic string, partition int32) {
	if e.wmStore == nil {
		return
	}
	wm, ok := e.wmStore.Watermark(topic, partition)
	if ok {
		session.MarkOffset(topic, partition, wm+1, "")
	}
}

// startWorkers 启动 worker 协程
func (e *asyncRetryEngine) startWorkers(ctx context.Context) {
	for i := 0; i < e.numWorkers; i++ {
		e.wg.Add(1)
		if e.wmStore != nil {
			go e.watermarkWorker(ctx)
		} else {
			go e.redisPollLoop(ctx)
		}
	}
}

// watermarkWorker 水位线模式的 worker
func (e *asyncRetryEngine) watermarkWorker(ctx context.Context) {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			if e.logger != nil {
				e.logger.Error("watermarkWorker panic recovered", "panic", r)
			}
		}
	}()

	for {
		item, err := e.waitForWatermarkItem(ctx)
		if item == nil {
			return
		}
		if err != nil {
			continue
		}

		e.processRetry(ctx, item)
	}
}

// waitForWatermarkItem 从 MemoryRetryStore 等待到期项
func (e *asyncRetryEngine) waitForWatermarkItem(ctx context.Context) (*RetryItem, error) {
	notifyCh := e.wmStore.Notify()
	for {
		items, err := e.store.Fetch(ctx, time.Now(), 1)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			return items[0], nil
		}

		// 等待通知或退出
		select {
		case <-notifyCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// redisPollLoop Redis 模式的轮询 worker
func (e *asyncRetryEngine) redisPollLoop(ctx context.Context) {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			if e.logger != nil {
				e.logger.Error("redisPollLoop panic recovered", "panic", r)
			}
		}
	}()

	const (
		minInterval   = 200 * time.Millisecond
		maxInterval   = 5 * time.Second
		backoffFactor = 2.0
	)

	interval := minInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			items, err := e.store.Fetch(ctx, time.Now(), 10)
			if err != nil {
				if e.logger != nil {
					e.logger.Error("fetch pending retries failed", "error", err)
				}
				continue
			}
			if len(items) > 0 {
				interval = minInterval
				for _, item := range items {
					e.processRetry(ctx, item)
				}
			} else {
				interval = time.Duration(float64(interval) * backoffFactor)
				if interval > maxInterval {
					interval = maxInterval
				}
			}
			ticker.Reset(interval)
		}
	}
}

// processRetry 处理一次重试（修复 P9：统一应用 handlerTimeout）
func (e *asyncRetryEngine) processRetry(ctx context.Context, item *RetryItem) {
	msgCtx, cancel := e.applyHandlerTimeout(ctx)
	defer cancel()

	err := e.handler.Handle(msgCtx, item.Topic, item.Value)

	if err == nil {
		// 重试成功
		if e.wmStore != nil {
			e.wmStore.MarkSuccess(item.Topic, item.Partition, item.Offset)
			session := e.getSession()
			if session != nil {
				e.commitWatermark(session, item.Topic, item.Partition)
			}
		} else {
			// Redis 模式：从 store 移除
			e.store.Remove(ctx, item)
		}
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
		if e.wmStore != nil {
			e.wmStore.RemovePending(item.Topic, item.Partition, item.Offset)
			session := e.getSession()
			if session != nil {
				e.commitWatermark(session, item.Topic, item.Partition)
			}
		} else {
			e.store.Remove(ctx, item)
		}
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
