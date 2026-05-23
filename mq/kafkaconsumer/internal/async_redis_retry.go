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

// asyncRedisRetry 异步重试 + Redis 持久化（Plan C）
//
// 消息首次尝试失败后持久化到 Redis，然后立即 MarkMessage 提交 offset（不阻塞 partition）。
// Worker 协程轮询 Redis 取出到期项并重试。
// 重启后从 Redis LoadAllPending 恢复未完成的重试。
type asyncRedisRetry struct {
	consumerGroup string
	maxRetry      uint
	backoff       retry.BackoffStrategy
	handler       func(ctx context.Context, topic string, msg []byte) error
	conf          *groupHandlerConf
	numWorkers    int
	logger        Logger
	metrics       MetricsCallbacks

	store RedisRetryStore

	wg       sync.WaitGroup
	shutdown chan struct{}
	started  bool
	workerMu sync.Mutex
}

func newAsyncRedisRetry(cg string, conf *groupHandlerConf, backoff retry.BackoffStrategy, numWorkers int, store RedisRetryStore) *asyncRedisRetry {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() * 2
	}
	return &asyncRedisRetry{
		consumerGroup: cg,
		maxRetry:      conf.MaxRetry,
		backoff:       backoff,
		handler:       conf.Handler,
		conf:          conf,
		numWorkers:    numWorkers,
		store:         store,
		shutdown:      make(chan struct{}),
	}
}

func (s *asyncRedisRetry) OnMessage(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	s.logger.Debug(fmt.Sprintf(
		"message claimed: cg=%q, topic=%q, partition=%d, offset=%d",
		s.consumerGroup, msg.Topic, msg.Partition, msg.Offset,
	))

	err := s.handler(ctx, msg.Topic, msg.Value)
	if err == nil {
		session.MarkMessage(msg, "")
		// 记录消费成功指标
		if s.metrics.OnConsume != nil {
			s.metrics.OnConsume(ctx, msg.Topic)
		}
		return
	}

	// maxRetry == 0 表示不重试，靠 Kafka 重投递保证至少一次语义。
	// 仅当 handleExhausted 成功处理（死信/失败回调执行完毕）时才提交 offset，
	// 否则不提交，让 Kafka 重投递此消息。
	if s.maxRetry == 0 {
		if s.conf.handleExhausted(ctx, s.consumerGroup, msg.Topic, msg.Value, err, s.logger, s.metrics) == exhaustedHandled {
			session.MarkMessage(msg, "")
		}
		return
	}

	// 记录重试指标
	if s.metrics.OnRetry != nil {
		s.metrics.OnRetry(ctx, msg.Topic)
	}

	// 持久化到 Redis
	nextRetryAt := time.Now().Add(s.backoff.Delay(0))
	item := SaramaMsgToRetryItem(msg, s.consumerGroup, 1, nextRetryAt)

	if storeErr := s.store.ScheduleRetry(session.Context(), item); storeErr != nil {
		// Redis 持久化失败，不提交 offset，让 Kafka 重投递
		s.logger.Error("schedule retry to redis failed, offset not committed",
			"topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset, "error", storeErr)
		return
	}

	// 持久化成功，提交 offset
	session.MarkMessage(msg, "")
}

func (s *asyncRedisRetry) SetSession(session sarama.ConsumerGroupSession) {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if !s.started {
		s.started = true
		s.startWorkers(session.Context())
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("recoverPending panic recovered", "panic", r)
				}
			}()
			s.recoverPending(session.Context())
		}()
	}
}

func (s *asyncRedisRetry) ClearSession() {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if s.started {
		close(s.shutdown)
		s.wg.Wait()
		s.shutdown = make(chan struct{})
		s.started = false
	}
}

func (s *asyncRedisRetry) OnShutdown(ctx context.Context) {
	close(s.shutdown)

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

func (s *asyncRedisRetry) SetLogger(l Logger) {
	s.logger = l
}

func (s *asyncRedisRetry) SetMetrics(m MetricsCallbacks) {
	s.metrics = m
}

// startWorkers 启动轮询 worker
func (s *asyncRedisRetry) startWorkers(ctx context.Context) {
	for i := 0; i < s.numWorkers; i++ {
		s.wg.Add(1)
		go s.pollLoop(ctx)
	}
}

// pollLoop 轮询 Redis 获取待重试项，自适应退避：有数据时快速轮询，空闲时逐步降低频率
func (s *asyncRedisRetry) pollLoop(ctx context.Context) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("pollLoop panic recovered", "panic", r)
		}
	}()

	const (
		minInterval   = 200 * time.Millisecond // 最快轮询间隔
		maxInterval   = 5 * time.Second        // 最慢轮询间隔
		backoffFactor = 2.0                    // 空闲退避倍率
	)

	interval := minInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			items, err := s.store.FetchPending(ctx, time.Now(), 10)
			if err != nil {
				s.logger.Error("fetch pending retries from redis failed", "error", err)
				continue
			}
			if len(items) > 0 {
				interval = minInterval // 有数据时快速轮询
				for _, item := range items {
					s.processRetry(ctx, item)
				}
			} else {
				// 无数据时逐步退避，降低 Redis 压力
				interval = time.Duration(float64(interval) * backoffFactor)
				if interval > maxInterval {
					interval = maxInterval
				}
			}
			ticker.Reset(interval)
		}
	}
}

// processRetry 处理一次 Redis 中的重试项
func (s *asyncRedisRetry) processRetry(ctx context.Context, item *RetryItem) {
	err := s.handler(ctx, item.Topic, item.Value)
	if err == nil {
		// 重试成功，从 Redis 移除
		if rmErr := s.store.RemoveRetry(ctx, item); rmErr != nil {
			s.logger.Error("remove retry item from redis failed",
				"topic", item.Topic, "offset", item.Offset, "error", rmErr)
		}
		// 记录消费成功指标（重试成功也属于最终消费成功）
		if s.metrics.OnConsume != nil {
			s.metrics.OnConsume(ctx, item.Topic)
		}
		return
	}

	if item.Attempt < s.maxRetry {
		// 还有重试次数，原子重调度
		// 记录重试指标
		if s.metrics.OnRetry != nil {
			s.metrics.OnRetry(ctx, item.Topic)
		}
		nextRetryAt := time.Now().Add(s.backoff.Delay(item.Attempt))
		newItem := &RetryItem{
			Topic:         item.Topic,
			Partition:     item.Partition,
			Offset:        item.Offset,
			Key:           item.Key,
			Value:         item.Value,
			Headers:       item.Headers,
			Attempt:       item.Attempt + 1,
			NextRetryAt:   nextRetryAt,
			ConsumerGroup: item.ConsumerGroup,
		}

		if rescheduleErr := s.store.AtomicReschedule(ctx, item, newItem); rescheduleErr != nil {
			s.logger.Error("atomic reschedule failed, keeping old item in redis",
				"topic", item.Topic, "offset", item.Offset, "error", rescheduleErr)
			// 原子重调度失败，旧项仍在 Redis 中，下次 poll 会重新取出
		}
		return
	}

	// 重试耗尽
	s.conf.handleExhausted(ctx, s.consumerGroup, item.Topic, item.Value, err, s.logger, s.metrics)
	if rmErr := s.store.RemoveRetry(ctx, item); rmErr != nil {
		s.logger.Error("remove exhausted retry item from redis failed",
			"topic", item.Topic, "offset", item.Offset, "error", rmErr)
	}
}

// recoverPending 启动时恢复 Redis 中的待重试项
func (s *asyncRedisRetry) recoverPending(ctx context.Context) {
	items, err := s.store.LoadAllPending(ctx)
	if err != nil {
		s.logger.Error("load pending retries from redis failed", "error", err)
		return
	}

	if len(items) == 0 {
		return
	}

	s.logger.Info("recovered pending retries from redis", "count", len(items))

	now := time.Now()
	for _, item := range items {
		// 到期项重新调度，使其可被 FetchPending 取出
		if item.NextRetryAt.Before(now) {
			item.NextRetryAt = now
		}
		if scheduleErr := s.store.ScheduleRetry(ctx, item); scheduleErr != nil {
			s.logger.Error("reschedule recovered item failed",
				"topic", item.Topic, "offset", item.Offset, "error", scheduleErr)
		}
	}
}
