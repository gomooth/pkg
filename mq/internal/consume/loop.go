// Package consume 提供消息队列的通用消费循环，适用于主动拉取模式（redis/httpsqs）。
// kafka 不使用此循环，其消费模式基于 sarama ConsumerGroup 回调。
package consume

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/telemetry"
	mqtraceutil "github.com/gomooth/pkg/mq/internal/traceutil"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// FetchResult 拉取结果
type FetchResult struct {
	Data  string // 消息内容
	Empty bool   // 队列为空
	Err   error  // 拉取错误
}

// Fetcher 消息拉取接口，由各 MQ 实现提供
type Fetcher interface {
	Fetch(ctx context.Context) FetchResult
}

// LoopConfig 消费循环配置
type LoopConfig struct {
	MQSystem      string // "redis" | "httpsqs"
	QueueName     string
	EmptySleep    time.Duration
	MaxErrors     uint
	PauseDuration time.Duration
	Backoff       retry.BackoffStrategy
	Tracer        trace.Tracer
}

// RetryStrategy 重试策略接口（消费循环层面）
type RetryStrategy interface {
	OnMessage(ctx context.Context, queue string, data []byte) error
}

// ConsumeLoop 通用消费循环，适用于 redis/httpsqs 的主动拉取模式。
// kafka 不使用此循环（其消费模式基于 sarama ConsumerGroup 回调）。
func ConsumeLoop(
	ctx context.Context,
	cfg LoopConfig,
	fetcher Fetcher,
	strategy RetryStrategy,
) {
	backoff := cfg.Backoff
	if backoff == nil {
		backoff = &retry.ExponentialDelay{Base: time.Second, Max: 30 * time.Second, Jitter: true}
	}

	pauseDuration := cfg.PauseDuration
	if pauseDuration == 0 {
		pauseDuration = 5 * time.Minute
	}
	maxErrors := cfg.MaxErrors
	if maxErrors == 0 {
		maxErrors = 50
	}

	tracer := cfg.Tracer
	if tracer == nil {
		tracer = telemetry.Tracer(fmt.Sprintf("mq.%s.consumer", cfg.MQSystem))
	}

	attempt := uint(0)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := fetcher.Fetch(ctx)
		if result.Err != nil {
			if ctx.Err() != nil {
				return
			}

			delay := backoff.Delay(attempt)
			attempt++

			if attempt >= maxErrors {
				select {
				case <-ctx.Done():
					return
				case <-time.After(pauseDuration):
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

		if result.Empty {
			attempt = 0
			sleepDur := cfg.EmptySleep
			if sleepDur == 0 {
				sleepDur = time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(sleepDur):
			}
			continue
		}

		// 成功获取消息，重置退避计数
		attempt = 0

		// Extract trace context
		msgCtx := mqtraceutil.ExtractTraceContext(ctx, result.Data)

		// Create consumer Span
		msgCtx, span := tracer.Start(msgCtx, fmt.Sprintf("%s consume", cfg.QueueName),
			trace.WithAttributes(
				attribute.String("messaging.system", cfg.MQSystem),
				attribute.String("messaging.destination", cfg.QueueName),
			),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)

		if err := strategy.OnMessage(msgCtx, cfg.QueueName, []byte(result.Data)); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if ctx.Err() != nil {
				span.End()
				return
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}
}
