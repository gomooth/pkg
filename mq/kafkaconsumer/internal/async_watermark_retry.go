package internal

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
)

// defaultMaxRetryQueueSize 重试队列默认最大容量，防止高错误率场景下内存无限增长
const defaultMaxRetryQueueSize = 10000

// asyncWatermarkRetry 异步重试 + 水位线 offset 跟踪（Plan B）
//
// 消息首次尝试失败后入内存优先队列，Worker 协程异步重试。
// 失败的消息不提交 offset，仅跟踪水位线：所有 <= 水位线的 offset 均已处理完成。
// 只提交水位线以内的 offset，保证重启后 Kafka 重投递未完成的消息。
type asyncWatermarkRetry struct {
	consumerGroup string
	maxRetry      uint
	backoff       retry.BackoffStrategy
	handler       func(ctx context.Context, topic string, msg []byte) error
	conf          *groupHandlerConf
	numWorkers    int
	maxQueueSize  int // 重试队列最大容量，0 表示无限制
	logger        Logger
	metrics       MetricsCallbacks

	watermark *WatermarkTracker

	// 优先队列
	pqMu   sync.Mutex
	pq     *retryHeap
	notify chan struct{} // 通知有新项入队

	// 当前 consumer group session 引用
	sessionMu sync.RWMutex
	session   sarama.ConsumerGroupSession

	// worker 生命周期管理
	workerMu sync.Mutex
	wg       sync.WaitGroup
	shutdown chan struct{}
	started  bool
}

func newAsyncWatermarkRetry(cg string, conf *groupHandlerConf, backoff retry.BackoffStrategy, numWorkers int) *asyncWatermarkRetry {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() * 2
	}
	maxQueueSize := conf.RetryMaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = defaultMaxRetryQueueSize
	}
	return &asyncWatermarkRetry{
		consumerGroup: cg,
		maxRetry:      conf.MaxRetry,
		backoff:       backoff,
		handler:       conf.Handler,
		conf:          conf,
		numWorkers:    numWorkers,
		maxQueueSize:  maxQueueSize,
		watermark:     NewWatermarkTracker(),
		pq:            newRetryHeap(maxQueueSize),
		notify:        make(chan struct{}, 1),
		shutdown:      make(chan struct{}),
	}
}

func (s *asyncWatermarkRetry) OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	s.logger.Debug(fmt.Sprintf(
		"message claimed: cg=%q, topic=%q, partition=%d, offset=%d",
		s.consumerGroup, msg.Topic, msg.Partition, msg.Offset,
	))

	err := s.handler(ctx, msg.Topic, msg.Value)
	if err == nil {
		s.watermark.MarkSuccess(msg.Topic, msg.Partition, msg.Offset)
		s.commitWatermark(session, msg.Topic, msg.Partition)
		// 记录消费成功指标
		if s.metrics.OnConsume != nil {
			s.metrics.OnConsume(ctx, msg.Topic)
		}
		return
	}

	// maxRetry == 0 表示不重试
	if s.maxRetry == 0 {
		if s.conf.handleExhausted(ctx, s.consumerGroup, msg.Topic, msg.Value, err, s.logger, s.metrics) == exhaustedHandled {
			s.watermark.RemovePending(msg.Topic, msg.Partition, msg.Offset)
			s.commitWatermark(session, msg.Topic, msg.Partition)
		}
		return
	}

	// 标记为 pending，不提交此 offset
	if !s.watermark.MarkPending(msg.Topic, msg.Partition, msg.Offset) {
		// pending 集合超限，降级为直接处理
		s.logger.Error("pending set overflow, degrading to exhausted handling",
			"topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset)
		if s.conf.handleExhausted(ctx, s.consumerGroup, msg.Topic, msg.Value, err, s.logger, s.metrics) == exhaustedHandled {
			s.watermark.RemovePending(msg.Topic, msg.Partition, msg.Offset)
			s.commitWatermark(session, msg.Topic, msg.Partition)
		}
		return
	}

	// 记录重试指标
	if s.metrics.OnRetry != nil {
		s.metrics.OnRetry(ctx, msg.Topic)
	}

	// 检查队列容量，超限时降级为直接处理（走死信/失败回调）并提交 offset
	s.pqMu.Lock()
	full := s.pq.IsFull()
	s.pqMu.Unlock()
	if full {
		s.logger.Error("retry queue capacity reached, degrading to exhausted handling",
			"topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset,
			"queue_size", s.pq.Len(), "max_queue_size", s.maxQueueSize)
		if s.conf.handleExhausted(ctx, s.consumerGroup, msg.Topic, msg.Value, err, s.logger, s.metrics) == exhaustedHandled {
			s.watermark.RemovePending(msg.Topic, msg.Partition, msg.Offset)
			s.commitWatermark(session, msg.Topic, msg.Partition)
		}
		return
	}

	// 入队异步重试
	nextRetryAt := time.Now().Add(s.backoff.Delay(0))
	item := SaramaMsgToRetryItem(msg, s.consumerGroup, 1, nextRetryAt)
	s.enqueue(item)
}

func (s *asyncWatermarkRetry) SetSession(session sarama.ConsumerGroupSession) {
	s.sessionMu.Lock()
	s.session = session
	s.sessionMu.Unlock()

	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if !s.started {
		s.started = true
		s.startWorkers(session.Context())
	}
}

func (s *asyncWatermarkRetry) ClearSession() {
	s.sessionMu.Lock()
	s.session = nil
	s.sessionMu.Unlock()

	s.workerMu.Lock()
	if s.started {
		// 停止现有 worker：关闭 shutdown channel 并等待退出
		close(s.shutdown)
		s.wg.Wait()

		// 重新初始化 channel，以备下次 SetSession 启动新 worker
		s.shutdown = make(chan struct{})
		s.notify = make(chan struct{}, 1)
		s.started = false
	}
	s.workerMu.Unlock()
}

func (s *asyncWatermarkRetry) OnShutdown(ctx context.Context) {
	close(s.shutdown)

	// 非阻塞通知 worker 检查退出
	select {
	case s.notify <- struct{}{}:
	default:
	}

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("wg.Wait goroutine panic recovered", "panic", r)
			}
		}()
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (s *asyncWatermarkRetry) SetLogger(l Logger) {
	s.logger = l
}

func (s *asyncWatermarkRetry) SetMetrics(m MetricsCallbacks) {
	s.metrics = m
}

// commitWatermark 提交水位线以内的 offset
func (s *asyncWatermarkRetry) commitWatermark(session sarama.ConsumerGroupSession, topic string, partition int32) {
	if wm, ok := s.watermark.Watermark(topic, partition); ok {
		// MarkOffset 提交 offset+1 作为下次消费起点
		session.MarkOffset(topic, partition, wm+1, "")
	}
}

// enqueue 将重试项加入优先队列并通知 worker
func (s *asyncWatermarkRetry) enqueue(item *RetryItem) {
	s.pqMu.Lock()
	s.pq.PushItem(item)
	s.pqMu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// startWorkers 启动 worker 协程
func (s *asyncWatermarkRetry) startWorkers(ctx context.Context) {
	for i := 0; i < s.numWorkers; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
}

// worker 从优先队列取出到期项并重试
func (s *asyncWatermarkRetry) worker(ctx context.Context) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("worker panic recovered", "panic", r)
		}
	}()

	for {
		item := s.waitForItem(ctx)
		if item == nil {
			return
		}

		s.processRetry(ctx, item)
	}
}

// waitForItem 阻塞等待优先队列中有到期的重试项，返回 nil 表示退出
func (s *asyncWatermarkRetry) waitForItem(ctx context.Context) *RetryItem {
	for {
		s.pqMu.Lock()

		// 检查退出信号（在锁内，确保与 shutdown 同步）
		select {
		case <-s.shutdown:
			s.pqMu.Unlock()
			return nil
		default:
		}

		peek := s.pq.Peek()
		if peek == nil {
			s.pqMu.Unlock()
			// 无锁等待通知
			select {
			case <-s.notify:
			case <-s.shutdown:
				return nil
			case <-ctx.Done():
				return nil
			}
			continue
		}

		now := time.Now()
		if peek.NextRetryAt.After(now) {
			waitDur := peek.NextRetryAt.Sub(now)
			if waitDur > time.Second {
				waitDur = time.Second
			}
			s.pqMu.Unlock()

			timer := time.NewTimer(waitDur)
			select {
			case <-timer.C:
			case <-s.notify:
				timer.Stop()
			case <-s.shutdown:
				timer.Stop()
				return nil
			case <-ctx.Done():
				timer.Stop()
				return nil
			}
			continue
		}

		item := s.pq.PopItem()
		s.pqMu.Unlock()
		return item
	}
}

// processRetry 处理一次重试
func (s *asyncWatermarkRetry) processRetry(ctx context.Context, item *RetryItem) {
	err := s.handler(ctx, item.Topic, item.Value)
	if err == nil {
		// 重试成功
		s.watermark.MarkSuccess(item.Topic, item.Partition, item.Offset)

		session := s.getSession()
		if session != nil {
			s.commitWatermark(session, item.Topic, item.Partition)
		}

		// 记录消费成功指标（重试成功也属于最终消费成功）
		if s.metrics.OnConsume != nil {
			s.metrics.OnConsume(ctx, item.Topic)
		}
		return
	}

	if item.Attempt < s.maxRetry {
		// 检查队列容量，超限时不再入队，直接走耗尽处理
		s.pqMu.Lock()
		full := s.pq.IsFull()
		s.pqMu.Unlock()
		if full {
			s.logger.Error("retry queue capacity reached during retry, degrading to exhausted handling",
				"topic", item.Topic, "partition", item.Partition, "offset", item.Offset,
				"attempt", item.Attempt, "queue_size", s.pq.Len(), "max_queue_size", s.maxQueueSize)
		} else {
			// 还有重试次数，重新入队
			// 记录重试指标
			if s.metrics.OnRetry != nil {
				s.metrics.OnRetry(ctx, item.Topic)
			}
			nextRetryAt := time.Now().Add(s.backoff.Delay(item.Attempt))
			item.Attempt++
			item.NextRetryAt = nextRetryAt
			s.enqueue(item)
			return
		}
	}

	// 重试耗尽
	if s.conf.handleExhausted(ctx, s.consumerGroup, item.Topic, item.Value, err, s.logger, s.metrics) == exhaustedHandled {
		s.watermark.RemovePending(item.Topic, item.Partition, item.Offset)
		session := s.getSession()
		if session != nil {
			s.commitWatermark(session, item.Topic, item.Partition)
		}
	}
}

func (s *asyncWatermarkRetry) getSession() sarama.ConsumerGroupSession {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	return s.session
}
