package kafkaproducer

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducer_Produce_Success(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 设置期望
	mockProducer.ExpectSendMessageAndSucceed()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试
	ctx := context.Background()
	topic := "test-topic"
	message := []byte("test message")

	err := p.Produce(ctx, topic, message)

	// 验证
	assert.NoError(t, err)
	assert.NoError(t, mockProducer.Close())
}

func TestProducer_ProduceWithSequence_Success(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 设置期望
	mockProducer.ExpectSendMessageAndSucceed()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试
	ctx := context.Background()
	topic := "test-topic"
	sequenceKey := "test-key"
	message := []byte("test message")

	err := p.ProduceWithSequence(ctx, topic, sequenceKey, message)

	// 验证
	assert.NoError(t, err)
	assert.NoError(t, mockProducer.Close())
}

func TestProducer_ProduceWithSequence_EmptyKey_Success(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 设置期望 - 空 key 也应该成功发送
	mockProducer.ExpectSendMessageAndSucceed()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试 - 空 key
	ctx := context.Background()
	topic := "test-topic"
	sequenceKey := ""
	message := []byte("test message")

	err := p.ProduceWithSequence(ctx, topic, sequenceKey, message)

	// 验证 - 空 key 不应该返回错误，只是没有 partition key
	assert.NoError(t, err)
	assert.NoError(t, mockProducer.Close())
}

func TestProducer_Produces_MultipleMessages(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 设置期望 - 多条消息
	mockProducer.ExpectSendMessageAndSucceed()
	mockProducer.ExpectSendMessageAndSucceed()
	mockProducer.ExpectSendMessageAndSucceed()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试
	ctx := context.Background()
	topic := "test-topic"
	messages := [][]byte{
		[]byte("message 1"),
		[]byte("message 2"),
		[]byte("message 3"),
	}

	err := p.Produces(ctx, topic, messages...)

	// 验证
	assert.NoError(t, err)
	assert.NoError(t, mockProducer.Close())
}

func TestProducer_ProduceWithSequence_MultipleMessages(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 设置期望 - 多条消息
	mockProducer.ExpectSendMessageAndSucceed()
	mockProducer.ExpectSendMessageAndSucceed()
	mockProducer.ExpectSendMessageAndSucceed()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试
	ctx := context.Background()
	topic := "test-topic"
	sequenceKey := "sequence-key"
	messages := [][]byte{
		[]byte("message 1"),
		[]byte("message 2"),
		[]byte("message 3"),
	}

	err := p.ProduceWithSequence(ctx, topic, sequenceKey, messages...)

	// 验证
	assert.NoError(t, err)
	assert.NoError(t, mockProducer.Close())
}

func TestProducer_ProduceWithSequence_NoMessages(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试 - 没有消息
	ctx := context.Background()
	topic := "test-topic"
	sequenceKey := "test-key"

	err := p.ProduceWithSequence(ctx, topic, sequenceKey)

	// 验证 - 应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no message")
}

func TestProducer_Close_Success(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 执行测试
	err := p.Close()

	// 验证
	assert.NoError(t, err)
}

func TestProducer_Close_NilProducer(t *testing.T) {
	// 创建 mock producer
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	// 直接构造 producer 结构体 - producer 为 nil
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      nil,
		producerReady: false,
		saramaConfig:  config,
	}

	// 执行测试
	err := p.Close()

	// 验证 - nil producer 不应该返回错误
	assert.NoError(t, err)
}

func TestProducer_Produce_ContextCancelled(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 创建已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 执行测试
	topic := "test-topic"
	message := []byte("test message")

	err := p.Produce(ctx, topic, message)

	// 验证 - 应该返回 context 错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestProducer_IProducerInterface(t *testing.T) {
	// 创建 mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	defer mockProducer.Close()

	// 直接构造 producer 结构体
	p := &producer{
		brokers:       []string{"localhost:9092"},
		producer:      mockProducer,
		producerReady: true,
	}

	// 验证实现了 IProducer 接口
	var _ IProducer = p

	ctx := context.Background()
	topic := "test-topic"
	message := []byte("test message")

	// 测试所有接口方法
	mockProducer.ExpectSendMessageAndSucceed()
	err := p.Produce(ctx, topic, message)
	require.NoError(t, err)

	mockProducer.ExpectSendMessageAndSucceed()
	mockProducer.ExpectSendMessageAndSucceed()
	err = p.Produces(ctx, topic, []byte("msg1"), []byte("msg2"))
	require.NoError(t, err)

	mockProducer.ExpectSendMessageAndSucceed()
	err = p.ProduceWithSequence(ctx, topic, "key", []byte("msg"))
	require.NoError(t, err)

	err = p.Close()
	require.NoError(t, err)
}

func TestProducer_NewWithCustomOptions(t *testing.T) {
	// 使用 New 函数创建 producer（不会连接真实 broker）
	p := New([]string{"localhost:9092", "localhost:9093"})

	// 验证基本属性
	assert.NotNil(t, p)
	// 注意：由于 New 返回的是 IProducer 接口，我们无法直接访问内部字段
	// 这个测试主要验证 New 函数不会 panic

	// 由于 producer 未初始化，调用 Produce 会超时或失败
	// 这里只是验证 New 函数的基本功能
	_ = p.Close()
}
