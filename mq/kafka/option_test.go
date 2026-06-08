package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gomooth/pkg/framework/retry"
	"github.com/gomooth/pkg/mq/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== ConsumerOption 测试 ====================

func TestConsumerOption_WithMaxRetry(t *testing.T) {
	cfg := consumerConfig{}
	WithMaxRetry(5)(&cfg)
	assert.Equal(t, 5, cfg.maxRetry)
}

func TestConsumerOption_WithBackoff(t *testing.T) {
	cfg := consumerConfig{}
	backoff := &retry.ExponentialDelay{Base: time.Second, Max: time.Minute}
	WithBackoff(backoff)(&cfg)
	assert.Equal(t, backoff, cfg.backoff)
}

func TestConsumerOption_WithHandlerTimeout(t *testing.T) {
	cfg := consumerConfig{}
	WithHandlerTimeout(30 * time.Second)(&cfg)
	assert.Equal(t, 30*time.Second, cfg.handlerTimeout)
}

func TestConsumerOption_WithPanicHandler(t *testing.T) {
	cfg := consumerConfig{}
	called := false
	fn := func(v any) { called = true }
	WithPanicHandler(fn)(&cfg)
	require.NotNil(t, cfg.panicHandler)
	cfg.panicHandler("test")
	assert.True(t, called)
}

func TestConsumerOption_WithRetryMode(t *testing.T) {
	cfg := consumerConfig{}
	WithRetryMode(RetryModeAsync)(&cfg)
	assert.Equal(t, types.RetryModeRequeue, cfg.retryMode) // RetryModeAsync maps to RetryModeRequeue
}

func TestConsumerOption_WithRetryWorkers(t *testing.T) {
	cfg := consumerConfig{}
	WithRetryWorkers(4)(&cfg)
	assert.Equal(t, 4, cfg.retryWorkers)
}

func TestConsumerOption_WithRetryMaxQueueSize(t *testing.T) {
	cfg := consumerConfig{}
	WithRetryMaxQueueSize(500)(&cfg)
	require.NotNil(t, cfg.retryStore)
	_, ok := cfg.retryStore.(*MemoryRetryStore)
	assert.True(t, ok)
}

func TestConsumerOption_WithSyncRetryMaxTotalTimeout(t *testing.T) {
	cfg := consumerConfig{}
	WithSyncRetryMaxTotalTimeout(2 * time.Minute)(&cfg)
	assert.Equal(t, 2*time.Minute, cfg.syncRetryMaxTotalTimeout)
}

func TestConsumerOption_WithRetryStore(t *testing.T) {
	cfg := consumerConfig{}
	store := NewMemoryRetryStore()
	WithRetryStore(store)(&cfg)
	assert.Equal(t, store, cfg.retryStore)
}

func TestConsumerOption_WithFailedHandler(t *testing.T) {
	cfg := consumerConfig{}
	called := false
	fn := types.FailedHandlerFunc(func(ctx context.Context, msg types.Message, err error) {
		called = true
	})
	WithFailedHandler(fn)(&cfg)
	require.NotNil(t, cfg.failedHandler)
	cfg.failedHandler(context.Background(), types.NewKafkaMessage("g", "t", nil), nil)
	assert.True(t, called)
}

func TestConsumerOption_WithConsumeGroupFailedHandler(t *testing.T) {
	cfg := consumerConfig{}
	fn1 := types.FailedHandlerFunc(func(ctx context.Context, msg types.Message, err error) {})
	fn2 := types.FailedHandlerFunc(func(ctx context.Context, msg types.Message, err error) {})
	WithConsumeGroupFailedHandler("group-a", fn1)(&cfg)
	WithConsumeGroupFailedHandler("group-b", fn2)(&cfg)
	require.NotNil(t, cfg.groupFailedHandlers)
	assert.NotNil(t, cfg.groupFailedHandlers["group-a"])
	assert.NotNil(t, cfg.groupFailedHandlers["group-b"])
}

func TestConsumerOption_WithConsumers(t *testing.T) {
	cfg := consumerConfig{}
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	reg1 := ConsumerRegistration{Group: "g1", Handler: handler, Topics: []string{"t1"}}
	reg2 := ConsumerRegistration{Group: "g2", Handler: handler, Topics: []string{"t2", "t3"}}
	WithConsumers(reg1, reg2)(&cfg)
	assert.Len(t, cfg.consumers, 2)
	assert.Equal(t, "g1", cfg.consumers[0].Group)
	assert.Equal(t, "g2", cfg.consumers[1].Group)
}

func TestConsumerOption_WithConsumer(t *testing.T) {
	cfg := consumerConfig{}
	handler := types.FuncHandler(func(ctx context.Context, msg types.Message) error { return nil })
	WithConsumer("my-group", handler, "topic1", "topic2", "topic3")(&cfg)
	require.Len(t, cfg.consumers, 1)
	assert.Equal(t, "my-group", cfg.consumers[0].Group)
	assert.Equal(t, []string{"topic1", "topic2", "topic3"}, cfg.consumers[0].Topics)
}

func TestConsumerOption_WithConsumerLogger(t *testing.T) {
	cfg := consumerConfig{}
	logger := newTestSlogLogger()
	WithConsumerLogger(logger)(&cfg)
	assert.Equal(t, logger, cfg.logger)
}

func TestConsumerOption_WithConsumerTimeout(t *testing.T) {
	cfg := consumerConfig{}
	WithConsumerTimeout(10 * time.Second)(&cfg)
	assert.Equal(t, 10*time.Second, cfg.timeout)
}

func TestConsumerOption_WithConsumerSaramaConfig(t *testing.T) {
	cfg := consumerConfig{}
	saramaCfg := sarama.NewConfig()
	WithConsumerSaramaConfig(saramaCfg)(&cfg)
	assert.Equal(t, saramaCfg, cfg.saramaConfig)
}

// ==================== ProducerOption 测试 ====================

func TestProducerOption_WithProducerTimeout(t *testing.T) {
	cfg := producerConfig{}
	WithProducerTimeout(10 * time.Second)(&cfg)
	assert.Equal(t, 10*time.Second, cfg.timeout)
}

func TestProducerOption_WithProducerLogger(t *testing.T) {
	cfg := producerConfig{}
	logger := newTestSlogLogger()
	WithProducerLogger(logger)(&cfg)
	assert.Equal(t, logger, cfg.logger)
}

func TestProducerOption_WithProducerSaramaConfig(t *testing.T) {
	cfg := producerConfig{}
	saramaCfg := sarama.NewConfig()
	WithProducerSaramaConfig(saramaCfg)(&cfg)
	assert.Equal(t, saramaCfg, cfg.saramaConfig)
}

// ==================== adaptFailedHandler 测试 ====================

func TestAdaptFailedHandler_Nil(t *testing.T) {
	result := adaptFailedHandler(nil)
	assert.Nil(t, result)
}

func TestAdaptFailedHandler_Adapts(t *testing.T) {
	var gotGroup, gotTopic string
	var gotMessage []byte
	oldFn := func(ctx context.Context, consumerGroup string, topic string, message []byte, err error) {
		gotGroup = consumerGroup
		gotTopic = topic
		gotMessage = message
	}
	adapted := adaptFailedHandler(oldFn)
	require.NotNil(t, adapted)

	msg := types.NewKafkaMessage("my-group", "my-topic", []byte("hello"))
	adapted(context.Background(), msg, nil)

	assert.Equal(t, "my-group", gotGroup)
	assert.Equal(t, "my-topic", gotTopic)
	assert.Equal(t, []byte("hello"), gotMessage)
}