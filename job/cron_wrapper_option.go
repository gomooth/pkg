package job

import (
	"log/slog"
	"time"
)

// cronJobWrapperOption 包装器选项的中间结构体
type cronJobWrapperOption struct {
	maxRetry     uint8
	timeout      time.Duration
	panicHandler PanicHandlerFunc
	failedSaver  func(jobName string, in []string, err error)
	log          *slog.Logger
}

// WrapperOption 定时任务包装器的配置选项
type WrapperOption func(*cronJobWrapperOption)

// WrapWithLogger 设置包装器的日志器
func WrapWithLogger(log *slog.Logger) WrapperOption {
	return func(o *cronJobWrapperOption) {
		o.log = log
	}
}

// WrapWithMaxRetry 设置最大重试次数
func WrapWithMaxRetry(retry uint8) WrapperOption {
	return func(o *cronJobWrapperOption) {
		o.maxRetry = retry
	}
}

// WrapWithFailedSaver 设置任务失败后的保存回调函数
func WrapWithFailedSaver(saver func(jobName string, in []string, err error)) WrapperOption {
	return func(o *cronJobWrapperOption) {
		o.failedSaver = saver
	}
}

// WrapWithTimeout 设置整体重试循环的超时时间，0 表示无超时（默认）
func WrapWithTimeout(d time.Duration) WrapperOption {
	return func(o *cronJobWrapperOption) {
		if d > 0 {
			o.timeout = d
		}
	}
}

// PanicHandlerFunc 定义 panic 恢复处理函数
type PanicHandlerFunc func(recover any)

// WrapWithPanicHandler 设置 panic 恢复处理器
func WrapWithPanicHandler(fn PanicHandlerFunc) WrapperOption {
	return func(o *cronJobWrapperOption) {
		if fn != nil {
			o.panicHandler = fn
		}
	}
}
