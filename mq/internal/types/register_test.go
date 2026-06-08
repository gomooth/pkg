package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ApplyRegisterOptions ---

func TestApplyRegisterOptions_Empty(t *testing.T) {
	// 无选项时返回零值配置
	cfg := ApplyRegisterOptions(nil)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Group)
	assert.Nil(t, cfg.ExtraTopics)
	assert.Nil(t, cfg.QueueOpts)
}

func TestApplyRegisterOptions_AllOptions(t *testing.T) {
	cfg := ApplyRegisterOptions([]RegisterOption{
		WithGroup("my-group"),
		WithExtraTopics("t1", "t2"),
		WithQueueOptions(
			WithQueueClient("fake-client"),
			WithQueueMaxRetry(3),
		),
	})
	assert.Equal(t, "my-group", cfg.Group)
	assert.Equal(t, []string{"t1", "t2"}, cfg.ExtraTopics)
	assert.Len(t, cfg.QueueOpts, 2)
}

// --- WithGroup ---

func TestWithGroup_SetsField(t *testing.T) {
	cfg := &RegisterConfig{}
	WithGroup("test-group")(cfg)
	assert.Equal(t, "test-group", cfg.Group)
}

func TestWithGroup_Empty(t *testing.T) {
	cfg := &RegisterConfig{}
	WithGroup("")(cfg)
	assert.Empty(t, cfg.Group)
}

// --- WithExtraTopics ---

func TestWithExtraTopics_SetsField(t *testing.T) {
	cfg := &RegisterConfig{}
	WithExtraTopics("topic-a", "topic-b")(cfg)
	assert.Equal(t, []string{"topic-a", "topic-b"}, cfg.ExtraTopics)
}

func TestWithExtraTopics_Appends(t *testing.T) {
	// 多次调用 WithExtraTopics 应追加而非覆盖
	cfg := &RegisterConfig{}
	WithExtraTopics("t1")(cfg)
	WithExtraTopics("t2", "t3")(cfg)
	assert.Equal(t, []string{"t1", "t2", "t3"}, cfg.ExtraTopics)
}

func TestWithExtraTopics_Empty(t *testing.T) {
	cfg := &RegisterConfig{}
	WithExtraTopics()(cfg)
	assert.Nil(t, cfg.ExtraTopics)
}

// --- WithQueueOptions ---

func TestWithQueueOptions_SetsField(t *testing.T) {
	opt := WithQueueMaxRetry(5)
	cfg := &RegisterConfig{}
	WithQueueOptions(opt)(cfg)
	assert.Len(t, cfg.QueueOpts, 1)
}

func TestWithQueueOptions_Appends(t *testing.T) {
	cfg := &RegisterConfig{}
	WithQueueOptions(WithQueueMaxRetry(1))(cfg)
	WithQueueOptions(WithQueueMaxRetry(2))(cfg)
	assert.Len(t, cfg.QueueOpts, 2)
}

// --- WithQueueClient ---

func TestWithQueueClient_SetsField(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueClient("my-client")(cfg)
	assert.Equal(t, "my-client", cfg.Client)
}

func TestWithQueueClient_Nil(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueClient(nil)(cfg)
	assert.Nil(t, cfg.Client)
}

// --- WithQueueMaxRetry ---

func TestWithQueueMaxRetry_SetsField(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueMaxRetry(10)(cfg)
	assert.NotNil(t, cfg.MaxRetry)
	assert.Equal(t, 10, *cfg.MaxRetry)
}

// --- WithQueueBackoff ---

func TestWithQueueBackoff_SetsField(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueBackoff("some-backoff-strategy")(cfg)
	assert.Equal(t, "some-backoff-strategy", cfg.Backoff)
}

func TestWithQueueBackoff_Nil(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueBackoff(nil)(cfg)
	assert.Nil(t, cfg.Backoff)
}

// --- WithQueueRetryMode ---

func TestWithQueueRetryMode_SetsField(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueRetryMode(RetryModeRequeue)(cfg)
	assert.NotNil(t, cfg.RetryMode)
	assert.Equal(t, RetryModeRequeue, *cfg.RetryMode)
}

func TestWithQueueRetryMode_Sync(t *testing.T) {
	cfg := &QueueConfig{}
	WithQueueRetryMode(RetryModeSync)(cfg)
	assert.NotNil(t, cfg.RetryMode)
	assert.Equal(t, RetryModeSync, *cfg.RetryMode)
}

// --- WithQueueFailedHandler ---

func TestWithQueueFailedHandler_SetsField(t *testing.T) {
	called := false
	fn := FailedHandlerFunc(func(ctx context.Context, msg Message, err error) {
		called = true
	})
	cfg := &QueueConfig{}
	WithQueueFailedHandler(fn)(cfg)
	assert.NotNil(t, cfg.FailedFn)

	// 调用验证
	cfg.FailedFn(context.Background(), Message{}, nil)
	assert.True(t, called)
}

// --- 集成：QueueOption 通过 RegisterOption 传递 ---

func TestApplyRegisterOptions_QueueOptionsIntegration(t *testing.T) {
	cfg := ApplyRegisterOptions([]RegisterOption{
		WithGroup("g1"),
		WithExtraTopics("extra1"),
		WithQueueOptions(
			WithQueueClient("client-1"),
			WithQueueMaxRetry(3),
			WithQueueBackoff("exponential"),
			WithQueueRetryMode(RetryModeRequeue),
			WithQueueFailedHandler(func(_ context.Context, _ Message, _ error) {}),
		),
	})

	assert.Equal(t, "g1", cfg.Group)
	assert.Equal(t, []string{"extra1"}, cfg.ExtraTopics)
	assert.Len(t, cfg.QueueOpts, 5)

	// 应用 QueueOpts 得到 QueueConfig
	qcfg := &QueueConfig{}
	for _, opt := range cfg.QueueOpts {
		opt(qcfg)
	}
	assert.Equal(t, "client-1", qcfg.Client)
	assert.Equal(t, 3, *qcfg.MaxRetry)
	assert.Equal(t, "exponential", qcfg.Backoff)
	assert.Equal(t, RetryModeRequeue, *qcfg.RetryMode)
	assert.NotNil(t, qcfg.FailedFn)
}
