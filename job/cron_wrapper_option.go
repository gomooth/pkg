package job

import (
	"log/slog"
	"time"
)

// WrapperOption 定时任务包装器的配置选项
type WrapperOption func(*cronJobWrapper)

// WrapWithLogger 设置包装器的日志器
func WrapWithLogger(log *slog.Logger) WrapperOption {
	return func(job *cronJobWrapper) {
		job.log = log
	}
}

// WrapWithMaxRetry 设置最大重试次数
func WrapWithMaxRetry(retry uint8) WrapperOption {
	return func(job *cronJobWrapper) {
		job.maxRetry = retry
	}
}

// WrapWithFailedSaver 设置任务失败后的保存回调函数
func WrapWithFailedSaver(saver func(jobName string, in []string, err error)) WrapperOption {
	return func(job *cronJobWrapper) {
		job.failedSaver = saver
	}
}

// WrapWithTimeout 设置整体重试循环的超时时间，0 表示无超时（默认）
func WrapWithTimeout(d time.Duration) WrapperOption {
	return func(job *cronJobWrapper) {
		if d > 0 {
			job.timeout = d
		}
	}
}
