package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobMetrics_Initialized(t *testing.T) {
	assert.NotNil(t, jobRunCounter)
	assert.NotNil(t, jobRetryCounter)
	assert.NotNil(t, jobRunDuration)
}

func TestRecordJobRun_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		recordJobRun(context.Background(), "test-job", "success", 100*time.Millisecond)
	})
	assert.NotPanics(t, func() {
		recordJobRun(context.Background(), "test-job", "error", 200*time.Millisecond)
	})
}

func TestRecordJobRetry_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		recordJobRetry(context.Background(), "test-job")
	})
}

func TestCommandJob_Run_MetricsRecordedOnSuccess(t *testing.T) {
	var calls int
	job := &commandJob{
		jobName: "test-metric-success",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				calls++
				return nil
			},
		},
		maxRetry: 0,
	}

	assert.NotPanics(t, func() { job.Run() })
	assert.Equal(t, 1, calls)
}

func TestCommandJob_Run_MetricsRecordedOnError(t *testing.T) {
	var calls int
	job := &commandJob{
		jobName: "test-metric-error",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				calls++
				return errors.New("fail")
			},
		},
		maxRetry: 1,
	}

	assert.NotPanics(t, func() { job.Run() })
	// 1次初始执行 + 1次重试 = 2次
	assert.Equal(t, 2, calls)
}

func TestCommandJob_Run_MetricsRecordedOnRetryThenSuccess(t *testing.T) {
	var calls int
	job := &commandJob{
		jobName: "test-metric-retry-success",
		job: &mockCommandJob{
			runFunc: func(ctx context.Context, args ...string) error {
				calls++
				if calls < 2 {
					return errors.New("fail before success")
				}
				return nil
			},
		},
		maxRetry: 2,
	}

	assert.NotPanics(t, func() { job.Run() })
	assert.Equal(t, 2, calls)
}
