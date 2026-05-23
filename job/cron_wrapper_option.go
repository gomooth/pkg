package job

import (
	"log/slog"
	"time"
)

type WrapperOption func(*cronJobWrapper)

func WrapWithLogger(log *slog.Logger) WrapperOption {
	return func(job *cronJobWrapper) {
		job.log = log
	}
}

func WrapWithMaxRetry(retry uint8) WrapperOption {
	return func(job *cronJobWrapper) {
		job.maxRetry = retry
	}
}

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
