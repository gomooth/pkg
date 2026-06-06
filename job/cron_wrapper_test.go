package job

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

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
