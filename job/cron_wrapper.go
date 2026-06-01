package job

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type cronJobWrapper struct {
	maxRetry uint8         // 最大重试次数
	timeout  time.Duration // 整体重试循环超时，0 表示无超时

	failedSaver func(jobName string, in []string, err error) // 错误记录器

	log *slog.Logger
}

// NewCronJobWrapper 创建定时任务包装器，通过选项函数配置重试、超时、日志等
func NewCronJobWrapper(opts ...WrapperOption) IWrapper {
	w := &cronJobWrapper{}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// FromCommandJob 将 ICommandJob 转换为 cron.Job，支持重试和超时控制
func (w *cronJobWrapper) FromCommandJob(_ context.Context, job ICommandJob, args ...string) cron.Job {
	name := "unknown"
	if job != nil {
		name = strings.Trim(fmt.Sprintf("%T", job), "*")
	}

	return &commandJob{
		jobName:     name,
		job:         job,
		args:        args,
		maxRetry:    w.maxRetry,
		timeout:     w.timeout,
		failedSaver: w.failedSaver,
		log:         w.log,
	}
}
