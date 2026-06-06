package kafka

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/metrics"
)

// groupHandler sarama.ConsumerGroupHandler 适配器（未导出）
type groupHandler struct {
	consumerGroup string
	handler       IHandler
	strategy      retryStrategy
	logger        *slog.Logger
}

// groupHandlerConf groupHandler 的配置
type groupHandlerConf struct {
	Logger                   *slog.Logger
	Handler                  IHandler
	MaxRetry                 int
	Backoff                  retry.BackoffStrategy
	FailedHandler            FailedHandlerFunc
	DeadLetter               DeadLetterHandler
	RetryMode                RetryMode
	RetryWorkers             int
	RetryStore               RetryStore
	Metrics                  *metrics.ConsumerMetrics
	HandlerTimeout           time.Duration
	SyncRetryMaxTotalTimeout time.Duration
}

func newGroupHandler(cg string, conf *groupHandlerConf) *groupHandler {
	backoff := conf.Backoff
	if backoff == nil {
		backoff = &retry.ExponentialDelay{Base: 10 * time.Second, Max: 5 * time.Minute}
	}

	logger := conf.Logger
	if logger == nil {
		logger = slog.Default()
	}
	internalLogger := logutil.NewSlogLogger(logger)

	// 注入默认 failedHandler（若用户未设置）
	failedHandler := conf.FailedHandler
	if failedHandler == nil {
		failedHandler = DefaultFailedHandlerFunc(internalLogger)
	}

	var strategy retryStrategy

	switch conf.RetryMode {
	case RetryModeAsync:
		store := conf.RetryStore
		if store == nil {
			// 默认使用 MemoryRetryStore（水位线模式）
			store = NewMemoryRetryStore()
		}
		numWorkers := conf.RetryWorkers
		if numWorkers <= 0 {
			numWorkers = runtime.NumCPU()
			if numWorkers < 1 {
				numWorkers = 1
			}
		}
		engine := newAsyncRetryEngineWithStore(cg, conf.Handler, conf.MaxRetry, backoff,
			conf.HandlerTimeout, numWorkers, store, internalLogger, conf.Metrics)
		engine.SetFailedHandler(failedHandler)
		engine.SetDeadLetterHandler(conf.DeadLetter)
		strategy = engine
	default: // RetryModeSync
		if conf.MaxRetry > 1 && logger != nil {
			logger.Warn("sync retry mode may block partition for extended periods, "+
				"consider using RetryModeAsync for production",
				"maxRetry", conf.MaxRetry, "syncRetryMaxTotalTimeout", conf.SyncRetryMaxTotalTimeout)
		}
		s := newSyncRetryStrategy(cg, conf.Handler, conf.MaxRetry, backoff,
			conf.SyncRetryMaxTotalTimeout, internalLogger, conf.Metrics)
		s.SetFailedHandler(failedHandler)
		s.SetDeadLetterHandler(conf.DeadLetter)
		strategy = s
	}

	return &groupHandler{
		consumerGroup: cg,
		handler:       conf.Handler,
		strategy:      strategy,
		logger:        logger,
	}
}

func (g *groupHandler) Setup(session sarama.ConsumerGroupSession) error {
	g.strategy.SetSession(session)
	return nil
}

func (g *groupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	g.strategy.ClearSession()
	return nil
}

func (g *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			// P8 修复：此处不输出 "message claimed" 调试日志

			// 从消息 headers 提取 trace context，创建消费者 Span
			ctx, span := startConsumerSpan(session.Context(), msg)
			g.strategy.OnMessage(ctx, session, msg)
			span.End()
		case <-session.Context().Done():
			return nil
		}
	}
}

// Shutdown 通知重试策略排空队列并关闭
func (g *groupHandler) Shutdown(ctx context.Context) {
	g.strategy.OnShutdown(ctx)
}
