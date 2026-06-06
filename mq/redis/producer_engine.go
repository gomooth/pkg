package redis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/mq/internal/metrics"
	mqtraceutil "github.com/gomooth/pkg/mq/internal/traceutil"
	"github.com/gomooth/pkg/mq/redis/internal"
	"github.com/gomooth/xerror"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	peIdle         int32 = 0
	peRunning      int32 = 1
	peShuttingDown int32 = 2
	peClosed       int32 = 3
)

// producerEngine 生产者生命周期引擎（未导出）
type producerEngine struct {
	addr  string
	opt   *producerConfig
	state atomic.Int32

	mu     sync.RWMutex
	client redis.UniversalClient

	logger  *slog.Logger
	metrics *metrics.ProducerMetrics
}

func newProducerEngine(addr string, cfg *producerConfig) *producerEngine {
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	return &producerEngine{
		addr:    addr,
		opt:     cfg,
		logger:  logger,
		metrics: metrics.NewProducerMetrics("redis"),
	}
}

func (e *producerEngine) Start(ctx context.Context) error {
	if !e.state.CompareAndSwap(peIdle, peRunning) {
		if e.state.Load() == peRunning {
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
		e.state.Store(peIdle)
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	e.mu.Lock()
	e.client = client
	e.mu.Unlock()

	return nil
}

func (e *producerEngine) Shutdown(ctx context.Context) error {
	if !e.state.CompareAndSwap(peRunning, peShuttingDown) {
		if e.state.Load() == peIdle {
			e.state.Store(peClosed)
		}
		return nil
	}

	e.mu.Lock()
	if e.client != nil {
		_ = e.client.Close()
		e.client = nil
	}
	e.mu.Unlock()

	e.state.Store(peClosed)
	return nil
}

func (e *producerEngine) Produce(ctx context.Context, queue string, message []byte) error {
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
		if e.metrics != nil {
			e.metrics.OnError()
		}
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	if e.metrics != nil {
		e.metrics.OnProduce(1)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (e *producerEngine) ProduceBatch(ctx context.Context, queue string, messages ...[]byte) error {
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
		if e.metrics != nil {
			e.metrics.OnError()
		}
		return xerror.WrapWithXCode(err, xcode.ErrMQPublish)
	}

	if e.metrics != nil {
		e.metrics.OnProduce(len(messages))
	}
	span.SetStatus(codes.Ok, "")
	return nil
}
