package job

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommandJob 用于测试的 mock 任务
type mockCommandJob struct {
	runFunc func(ctx context.Context, args ...string) error
}

func (m *mockCommandJob) Run(ctx context.Context, args ...string) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, args...)
	}
	return nil
}

func TestCommandJob_Run_SucceedsOnFirstAttempt(t *testing.T) {
	var calls atomic.Int32
	job := &commandJob{
		jobName: "test-success",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				calls.Add(1)
				return nil
			},
		},
		maxRetry: 2,
	}

	job.Run()

	assert.Equal(t, int32(1), calls.Load(), "应只调用一次")
}

func TestCommandJob_Run_RetriesUpToMaxRetry(t *testing.T) {
	var calls atomic.Int32
	job := &commandJob{
		jobName: "test-retry",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				calls.Add(1)
				return errors.New("always fail")
			},
		},
		maxRetry: 2,
	}

	job.Run()

	// maxRetry=2 意味着最多 3 次尝试（1次初始 + 2次重试）
	assert.Equal(t, int32(3), calls.Load(), "应重试到 maxRetry 次数")
}

func TestCommandJob_Run_SucceedsAfterRetries(t *testing.T) {
	var calls atomic.Int32
	job := &commandJob{
		jobName: "test-retry-then-success",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				c := calls.Add(1)
				if c < 3 {
					return errors.New("fail before success")
				}
				return nil
			},
		},
		maxRetry: 3,
	}

	job.Run()

	// 第3次成功，不再重试
	assert.Equal(t, int32(3), calls.Load())
}

func TestCommandJob_Run_ContextCancellation(t *testing.T) {
	var calls atomic.Int32

	// 由于 commandJob.Run() 内部创建自己的 context，
	// 通过 timeout 方式触发 context 取消
	job := &commandJob{
		jobName: "test-timeout",
		job: &mockCommandJob{
			runFunc: func(innerCtx context.Context, args ...string) error {
				calls.Add(1)
				return errors.New("fail")
			},
		},
		maxRetry: 10,
		timeout:  1, // 1纳秒超时，几乎立即取消
	}

	job.Run()

	// 超时后应停止重试，调用次数应远小于 11
	assert.Less(t, calls.Load(), int32(11), "超时后应停止重试")
}

func TestCommandJob_Run_FailedSaverCalled(t *testing.T) {
	var saverCalled atomic.Int32
	var savedJobName string
	var savedErr error

	job := &commandJob{
		jobName: "test-failed-saver",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				return errors.New("always fail")
			},
		},
		maxRetry: 1,
		failedSaver: func(jobName string, in []string, err error) {
			saverCalled.Add(1)
			savedJobName = jobName
			savedErr = err
		},
	}

	job.Run()

	assert.Equal(t, int32(1), saverCalled.Load(), "failedSaver 应被调用")
	assert.Equal(t, "test-failed-saver", savedJobName)
	assert.Error(t, savedErr)
}

func TestCommandJob_Run_FailedSaverNotCalledOnSuccess(t *testing.T) {
	var saverCalled atomic.Int32

	job := &commandJob{
		jobName: "test-no-saver",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				return nil
			},
		},
		maxRetry: 2,
		failedSaver: func(jobName string, in []string, err error) {
			saverCalled.Add(1)
		},
	}

	job.Run()

	assert.Equal(t, int32(0), saverCalled.Load(), "成功时 failedSaver 不应被调用")
}

func TestCommandJob_Run_FailedSaverNil(t *testing.T) {
	// 确保 failedSaver 为 nil 时不 panic
	job := &commandJob{
		jobName: "test-nil-saver",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				return errors.New("fail")
			},
		},
		maxRetry:    1,
		failedSaver: nil,
	}

	assert.NotPanics(t, func() {
		job.Run()
	})
}

func TestFromCommandJob_ProducesValidCronJob(t *testing.T) {
	wrapper := NewCronJobWrapper()

	tests := []struct {
		name      string
		job       ICommandJob
		args      []string
		wantPanic bool
	}{
		{
			name:      "正常任务",
			job:       &mockCommandJob{},
			args:      []string{"arg1", "arg2"},
			wantPanic: false,
		},
		{
			name:      "无参数任务",
			job:       &mockCommandJob{},
			args:      nil,
			wantPanic: false,
		},
		{
			name:      "nil 任务通过 panic handler 恢复",
			job:       nil,
			args:      nil,
			wantPanic: false, // panic 被 recover 捕获,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cronJob := wrapper.FromCommandJob(ctx, tt.job, tt.args...)

			// 应实现 cron.Job 接口
			var _ cron.Job = cronJob

			if tt.wantPanic {
				assert.Panics(t, func() {
					cronJob.Run()
				})
			} else {
				assert.NotPanics(t, func() {
					cronJob.Run()
				})
			}
		})
	}
}

func TestFromCommandJob_WithWrapperOptions(t *testing.T) {
	var saverCalled atomic.Int32

	wrapper := NewCronJobWrapper(
		WrapWithMaxRetry(1),
		WrapWithFailedSaver(func(jobName string, in []string, err error) {
			saverCalled.Add(1)
		}),
	)

	ctx := context.Background()
	cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{
		runFunc: func(ctx context.Context, args ...string) error {
			return errors.New("fail")
		},
	}, "arg1")

	require.NotNil(t, cronJob)

	cronJob.Run()

	assert.Equal(t, int32(1), saverCalled.Load(), "通过 FromCommandJob 创建的任务应使用 wrapper 选项")
}

func TestFromCommandJob_JobName(t *testing.T) {
	wrapper := NewCronJobWrapper()
	ctx := context.Background()

	t.Run("non-nil job gets type name", func(t *testing.T) {
		cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{})
		require.NotNil(t, cronJob)

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		// 类型名应包含 mockCommandJob（去掉指针前缀）
		assert.Contains(t, cj.jobName, "mockCommandJob")
	})

	t.Run("nil job gets unknown name", func(t *testing.T) {
		cronJob := wrapper.FromCommandJob(ctx, nil)
		require.NotNil(t, cronJob)

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		assert.Equal(t, "unknown", cj.jobName)
	})
}

func TestCommandJob_Run_WithArgs(t *testing.T) {
	var receivedArgs []string
	job := &commandJob{
		jobName: "test-args",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				receivedArgs = args
				return nil
			},
		},
		args:     []string{"hello", "world"},
		maxRetry: 0,
	}

	job.Run()

	assert.Equal(t, []string{"hello", "world"}, receivedArgs)
}

// ---------- WrapWithLogger 选项函数测试 ----------

func TestWrapWithLogger(t *testing.T) {
	t.Run("sets logger on cronJobWrapper", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		wrapper := NewCronJobWrapper(WrapWithLogger(logger))
		ctx := context.Background()
		cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				return nil
			},
		})
		require.NotNil(t, cronJob)

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		assert.NotNil(t, cj.log, "log 应被 WrapWithLogger 设置")
	})

	t.Run("logger outputs through injected logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		job := &commandJob{
			jobName:  "test-logger-output",
			job:      &mockCommandJob{},
			maxRetry: 0,
			log:      logger,
		}

		job.Run()

		assert.Contains(t, buf.String(), "test-logger-output", "日志应通过注入的 logger 输出")
		assert.Contains(t, buf.String(), "run starting", "应记录开始日志")
	})
}

// ---------- WrapWithTimeout 选项函数测试 ----------

func TestWrapWithTimeout(t *testing.T) {
	t.Run("d > 0 sets timeout", func(t *testing.T) {
		wrapper := NewCronJobWrapper(WrapWithTimeout(5 * time.Second))
		ctx := context.Background()
		cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{})

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		assert.Equal(t, 5*time.Second, cj.timeout, "timeout 应被设置为 5s")
	})

	t.Run("d == 0 does not set timeout", func(t *testing.T) {
		wrapper := NewCronJobWrapper(WrapWithTimeout(0))
		ctx := context.Background()
		cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{})

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		assert.Equal(t, time.Duration(0), cj.timeout, "d==0 时不应设置 timeout")
	})

	t.Run("timeout triggers DeadlineExceeded error path", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		job := &commandJob{
			jobName: "test-timeout-path",
			job: &mockCommandJob{
				runFunc: func(ctx context.Context, args ...string) error {
					return errors.New("fail")
				},
			},
			maxRetry: 10,
			timeout:  1 * time.Nanosecond,
			log:      logger,
		}

		job.Run()

		assert.Contains(t, buf.String(), "timed out", "超时时应记录 timed out 日志")
	})
}

// ---------- WrapWithPanicHandler 选项函数测试 ----------

func TestWrapWithPanicHandler(t *testing.T) {
	t.Run("non-nil handler is called on panic via wrapper option", func(t *testing.T) {
		var handlerCalled atomic.Int32
		var recoveredVal any

		handler := func(r any) {
			handlerCalled.Add(1)
			recoveredVal = r
		}

		wrapper := NewCronJobWrapper(WrapWithPanicHandler(handler))
		ctx := context.Background()
		cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				panic("test panic value")
			},
		})

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		assert.NotNil(t, cj.panicHandler, "通过 WrapWithPanicHandler 设置的 handler 应存在")

		assert.NotPanics(t, func() {
			cronJob.Run()
		})

		assert.Equal(t, int32(1), handlerCalled.Load(), "panicHandler 应被调用")
		assert.Equal(t, "test panic value", recoveredVal, "应接收到 panic 值")
	})

	t.Run("non-nil handler is called on panic via direct construction", func(t *testing.T) {
		var handlerCalled atomic.Int32
		var recoveredVal any

		handler := func(r any) {
			handlerCalled.Add(1)
			recoveredVal = r
		}

		job := &commandJob{
			jobName: "test-panic-handler",
			job: &mockCommandJob{
				runFunc: func(ctx context.Context, args ...string) error {
					panic("test panic value")
				},
			},
			maxRetry:     0,
			panicHandler: handler,
		}

		assert.NotPanics(t, func() {
			job.Run()
		})

		assert.Equal(t, int32(1), handlerCalled.Load(), "panicHandler 应被调用")
		assert.Equal(t, "test panic value", recoveredVal, "应接收到 panic 值")
	})

	t.Run("nil handler does not affect default behavior", func(t *testing.T) {
		wrapper := NewCronJobWrapper(WrapWithPanicHandler(nil))
		ctx := context.Background()
		cronJob := wrapper.FromCommandJob(ctx, &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				panic("test panic")
			},
		})

		cj, ok := cronJob.(*commandJob)
		require.True(t, ok)
		assert.Nil(t, cj.panicHandler, "nil handler 不应设置 panicHandler")

		// 不应 panic（默认 recover 捕获）
		assert.NotPanics(t, func() {
			cronJob.Run()
		})
	})

	t.Run("panic without handler logs error", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		job := &commandJob{
			jobName: "test-panic-no-handler",
			job: &mockCommandJob{
				runFunc: func(ctx context.Context, args ...string) error {
					panic("oops")
				},
			},
			maxRetry: 0,
			log:      logger,
		}

		job.Run()

		assert.Contains(t, buf.String(), "panic", "无 panicHandler 时应记录 panic 日志")
		assert.Contains(t, buf.String(), "oops", "日志应包含 panic 值")
	})
}

// ---------- logf 分支覆盖测试 ----------

func TestLogf_WithLogger(t *testing.T) {
	t.Run("debug level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		job := commandJob{jobName: "log-test", log: logger}
		job.logf("debug", "debug message %d", 1)
		assert.Contains(t, buf.String(), "debug message 1")
	})

	t.Run("info level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		job := commandJob{jobName: "log-test", log: logger}
		job.logf("info", "info message %d", 2)
		assert.Contains(t, buf.String(), "info message 2")
	})

	t.Run("warn level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		job := commandJob{jobName: "log-test", log: logger}
		job.logf("warn", "warn message %d", 3)
		assert.Contains(t, buf.String(), "warn message 3")
	})

	t.Run("error level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		job := commandJob{jobName: "log-test", log: logger}
		job.logf("error", "error message %d", 4)
		assert.Contains(t, buf.String(), "error message 4")
	})

	t.Run("err level alias", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		job := commandJob{jobName: "log-test", log: logger}
		job.logf("err", "err alias message")
		assert.Contains(t, buf.String(), "err alias message")
	})
}

func TestLogf_WithoutLogger(t *testing.T) {
	t.Run("no panic with nil logger debug", func(t *testing.T) {
		job := commandJob{jobName: "log-test", log: nil}
		assert.NotPanics(t, func() {
			job.logf("debug", "no logger debug")
		})
	})

	t.Run("no panic with nil logger info", func(t *testing.T) {
		job := commandJob{jobName: "log-test", log: nil}
		assert.NotPanics(t, func() {
			job.logf("info", "no logger info")
		})
	})

	t.Run("no panic with nil logger warn", func(t *testing.T) {
		job := commandJob{jobName: "log-test", log: nil}
		assert.NotPanics(t, func() {
			job.logf("warn", "no logger warn")
		})
	})

	t.Run("no panic with nil logger error", func(t *testing.T) {
		job := commandJob{jobName: "log-test", log: nil}
		assert.NotPanics(t, func() {
			job.logf("error", "no logger error")
		})
	})

	t.Run("no panic with nil logger err alias", func(t *testing.T) {
		job := commandJob{jobName: "log-test", log: nil}
		assert.NotPanics(t, func() {
			job.logf("err", "no logger err")
		})
	})
}

// ---------- commandJob.Run 边界路径测试 ----------

func TestCommandJob_Run_PanicRecoveryWithHandler(t *testing.T) {
	var handlerCalled atomic.Int32
	var recoveredVal any

	job := &commandJob{
		jobName: "test-panic-recover",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				panic("boom")
			},
		},
		maxRetry: 0,
		panicHandler: func(r any) {
			handlerCalled.Add(1)
			recoveredVal = r
		},
	}

	assert.NotPanics(t, func() {
		job.Run()
	})

	assert.Equal(t, int32(1), handlerCalled.Load(), "panicHandler 应被调用一次")
	assert.Equal(t, "boom", recoveredVal)
}

func TestCommandJob_Run_ContextDeadlineExceeded(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 使用极短超时 + 阻塞式 job 来触发 DeadlineExceeded
	job := &commandJob{
		jobName: "test-deadline-exceeded",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				// 等待 context 超时
				<-ctx.Done()
				return ctx.Err()
			},
		},
		maxRetry: 1,
		timeout:  10 * time.Millisecond,
		log:      logger,
	}

	job.Run()

	assert.Contains(t, buf.String(), "timed out", "应记录超时日志")
}

func TestCommandJob_Run_ContextCancelled(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	// 立即取消 context
	cancel()

	job := &commandJob{
		jobName: "test-cancelled",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				return context.Canceled
			},
		},
		maxRetry: 1,
		ctx:      ctx,
		log:      logger,
	}

	job.Run()

	assert.Contains(t, buf.String(), "cancelled", "应记录取消日志")
}
