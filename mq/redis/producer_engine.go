package redis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/internal/engine"
	"github.com/gomooth/pkg/mq/internal/metrics"
	mqtraceutil "github.com/gomooth/pkg/mq/internal/traceutil"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/gomooth/pkg/mq/redis/internal"
	"github.com/gomooth/xerror"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// producerEngine 生产者生命周期引擎（未导出）
type producerEngine struct {
	engine.Base
	addr string
	opt  *producerConfig

	mu     sync.RWMutex
	client redis.UniversalClient
}

func newProducerEngine(addr string, cfg *producerConfig) *producerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	eng := &producerEngine{
		Base: engine.Base{
			Logger:  logger,
			Metrics: metrics.NewProducerMetrics("redis"),
		},
		addr: addr,
		opt:  cfg,
	}
	return eng
}

func (e *producerEngine) Start(ctx context.Context) error {
	if !e.TryStart() {
		if e.State.Load() == engine.Running {
			return nil
		}
		return xerror.NewXCode(xcode.ErrMQPublish, "producer already closed")
	}

	opts := e.opt.redisOptions
	if opts == nil {
		opts = internal.BuildProducerOptions(e.addr)
	}
	opts.Addr = e.addr

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		e.State.Store(engine.Idle)
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	e.mu.Lock()
	e.client = client
	e.mu.Unlock()

	return nil
}

func (e *producerEngine) Shutdown(ctx context.Context) error {
	if !e.RequestShutdown() {
		if e.State.Load() == engine.Idle {
			e.State.Store(engine.Closed)
		}
		return nil
	}

	e.mu.Lock()
	if e.client != nil {
		_ = e.client.Close()
		e.client = nil
	}
	e.mu.Unlock()

	e.State.Store(engine.Closed)
	return nil
}

func (e *producerEngine) Produce(ctx context.Context, queue string, message []byte, opts ...types.ProduceOption) error {
	if err := ctx.Err(); err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	// Create producer Span
	tracer := telemetry.Tracer("mq.redis.producer")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("%s produce", queue),
		trace.WithAttributes(
			attribute.String("messaging.system", "redis"),
			attribute.String("messaging.destination", queue),
		),
		trace.WithSpanKind(trace.SpanKindProducer),
	)
	defer span.End()

	// Inject trace context into message
	injectedMsg := mqtraceutil.InjectTraceContext(ctx, string(message))

	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()

	if client == nil {
		span.RecordError(fmt.Errorf("producer not connected"))
		span.SetStatus(codes.Error, "producer not connected")
		return xerror.NewXCode(xcode.ErrMQPublish, "producer not connected")
	}

	queueKey := fmt.Sprintf("%s%s", e.opt.queuePrefix, queue)
	if err := client.LPush(ctx, queueKey, []byte(injectedMsg)).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if m, ok := e.Metrics.(*metrics.ProducerMetrics); ok && m != nil {
			m.OnError()
		}
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	if m, ok := e.Metrics.(*metrics.ProducerMetrics); ok && m != nil {
		m.OnProduce(1)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (e *producerEngine) ProduceBatch(ctx context.Context, queue string, messages [][]byte, opts ...types.ProduceOption) error {
	if len(messages) == 0 {
		return xerror.NewXCode(xcode.ErrMQPublish, "no messages")
	}

	if err := ctx.Err(); err != nil {
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	// Create producer Span
	tracer := telemetry.Tracer("mq.redis.producer")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("%s produce batch", queue),
		trace.WithAttributes(
			attribute.String("messaging.system", "redis"),
			attribute.String("messaging.destination", queue),
			attribute.Int("messaging.batch.size", len(messages)),
		),
		trace.WithSpanKind(trace.SpanKindProducer),
	)
	defer span.End()

	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()

	if client == nil {
		span.RecordError(fmt.Errorf("producer not connected"))
		span.SetStatus(codes.Error, "producer not connected")
		return xerror.NewXCode(xcode.ErrMQPublish, "producer not connected")
	}

	queueKey := fmt.Sprintf("%s%s", e.opt.queuePrefix, queue)

	// 使用 Pipeline 批量推送，注入 trace context
	pipe := client.Pipeline()
	for _, msg := range messages {
		injectedMsg := mqtraceutil.InjectTraceContext(ctx, string(msg))
		pipe.LPush(ctx, queueKey, []byte(injectedMsg))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if m, ok := e.Metrics.(*metrics.ProducerMetrics); ok && m != nil {
			m.OnError()
		}
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	if m, ok := e.Metrics.(*metrics.ProducerMetrics); ok && m != nil {
		m.OnProduce(len(messages))
	}
	span.SetStatus(codes.Ok, "")
	return nil
}
