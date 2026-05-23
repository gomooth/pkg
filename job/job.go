package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gomooth/pkg/framework/retry"
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
	defer j.logf("debug", "[job] %s run end", j.jobName)

	cfg := retry.Config{
		MaxAttempts: uint(j.maxRetry) + 1,
		Strategy:    &retry.LinearDelay{Base: 1e9},
	}

	ctx := context.Background()
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
			// 区分 job 自身错误和 context 取消
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				j.logf("warn", "[job] %s cancelled (attempt %d/%d): %v", j.jobName, attempt+1, cfg.MaxAttempts, ctx.Err())
			} else {
				j.logf("warn", "[job] %s run failed (attempt %d/%d): %+v", j.jobName, attempt+1, cfg.MaxAttempts, err)
			}
		}
		return err
	})

	if lastErr != nil {
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
