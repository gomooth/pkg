package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/framework/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type commandJob struct {
	jobName string
	job     ICommandJob
	args    []string

	maxRetry uint8         // 最大重试次数
	timeout  time.Duration // 整体重试循环超时，0 表示无超时

	failedSaver func(jobName string, in []string, err error)

	log *slog.Logger
}

func (j commandJob) Run() {
	j.logf("debug", "[job] %s run starting", j.jobName)
	start := time.Now()
	defer j.logf("debug", "[job] %s run end", j.jobName)

	tracer := telemetry.Tracer("job")

	ctx, span := tracer.Start(context.Background(), j.jobName,
		trace.WithAttributes(
			attribute.String("job.name", j.jobName),
			attribute.String("job.type", "cron"),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	cfg := retry.Config{
		MaxAttempts: uint(j.maxRetry) + 1,
		Strategy:    &retry.LinearDelay{Base: 1e9},
	}

	var cancel context.CancelFunc
	if j.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, j.timeout)
		defer cancel()
	}

	lastErr := retry.Do(ctx, cfg, func(attempt uint) error {
		// 每次重试前再次检查 context，避免无效执行
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := j.job.Run(ctx, j.args...)
		if err != nil {
			// 记录重试指标（不含首次执行）
			if attempt > 0 {
				recordJobRetry(ctx, j.jobName)
			}
			// 区分 job 自身错误和 context 取消
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				j.logf("warn", "[job] %s cancelled (attempt %d/%d): %v", j.jobName, attempt+1, cfg.MaxAttempts, ctx.Err())
			} else {
				j.logf("warn", "[job] %s run failed (attempt %d/%d): %+v", j.jobName, attempt+1, cfg.MaxAttempts, err)
			}
		}
		return err
	})

	duration := time.Since(start)
	if lastErr != nil {
		span.RecordError(lastErr)
		span.SetStatus(codes.Error, lastErr.Error())
		recordJobRun(ctx, j.jobName, "error", duration)
		if errors.Is(lastErr, context.DeadlineExceeded) {
			j.logf("error", "[job] %s timed out after %v", j.jobName, j.timeout)
		} else if errors.Is(lastErr, context.Canceled) {
			j.logf("error", "[job] %s cancelled", j.jobName)
		} else {
			j.logf("error", "[job] %s run failed after %d attempts: %+v", j.jobName, cfg.MaxAttempts, lastErr)
		}
		if j.failedSaver != nil {
			j.failedSaver(j.jobName, j.args, lastErr)
		}
	} else {
		span.SetStatus(codes.Ok, "")
		recordJobRun(ctx, j.jobName, "success", duration)
	}
}

func (j commandJob) logf(level string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	comp := slog.String("component", "job")

	if j.log == nil {
		switch level {
		case "debug":
			slog.Debug(msg, comp)
		case "info":
			slog.Info(msg, comp)
		case "warn":
			slog.Warn(msg, comp)
		case "error", "err":
			slog.Error(msg, comp)
		}
		return
	}

	switch level {
	case "debug":
		j.log.Debug(msg, comp)
	case "info":
		j.log.Info(msg, comp)
	case "warn":
		j.log.Warn(msg, comp)
	case "error", "err":
		j.log.Error(msg, comp)
	}
}
