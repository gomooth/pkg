package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- 构造函数测试：验证 system 标志正确 ---

func TestNewRedisMessage(t *testing.T) {
	msg := NewRedisMessage("my-queue", []byte("hello"))
	assert.Equal(t, "my-queue", msg.Queue)
	assert.Equal(t, []byte("hello"), msg.Data)
	assert.True(t, msg.IsRedis())
	assert.False(t, msg.IsKafka())
	assert.False(t, msg.IsHttpsqs())
}

func TestNewKafkaMessage(t *testing.T) {
	msg := NewKafkaMessage("my-group", "my-topic", []byte("hello"))
	assert.Equal(t, "my-topic", msg.Queue)
	assert.Equal(t, []byte("hello"), msg.Data)
	assert.Equal(t, "my-group", msg.Group)
	assert.True(t, msg.IsKafka())
	assert.False(t, msg.IsRedis())
	assert.False(t, msg.IsHttpsqs())
}

func TestNewHttpsqSMessage(t *testing.T) {
	msg := NewHttpsqSMessage("my-queue", []byte("hello"), 42)
	assert.Equal(t, "my-queue", msg.Queue)
	assert.Equal(t, []byte("hello"), msg.Data)
	assert.Equal(t, int64(42), msg.Pos)
	assert.True(t, msg.IsHttpsqs())
	assert.False(t, msg.IsRedis())
	assert.False(t, msg.IsKafka())
}

// --- 类型检查方法 ---

func TestMessage_IsRedis(t *testing.T) {
	assert.True(t, NewRedisMessage("q", nil).IsRedis())
	assert.False(t, NewKafkaMessage("g", "t", nil).IsRedis())
	assert.False(t, NewHttpsqSMessage("q", nil, 1).IsRedis())
}

func TestMessage_IsKafka(t *testing.T) {
	assert.True(t, NewKafkaMessage("g", "t", nil).IsKafka())
	assert.False(t, NewRedisMessage("q", nil).IsKafka())
	assert.False(t, NewHttpsqSMessage("q", nil, 1).IsKafka())
}

func TestMessage_IsHttpsqs(t *testing.T) {
	assert.True(t, NewHttpsqSMessage("q", nil, 1).IsHttpsqs())
	assert.False(t, NewRedisMessage("q", nil).IsHttpsqs())
	assert.False(t, NewKafkaMessage("g", "t", nil).IsHttpsqs())
}

// --- 类型化访问器：在错误的 MQ 类型上调用时返回错误 ---

func TestMessage_KafkaGroup_WrongType(t *testing.T) {
	msg := NewRedisMessage("q", nil)
	_, err := msg.KafkaGroup()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "KafkaGroup()")
	assert.Contains(t, err.Error(), "redis")
}

func TestMessage_KafkaGroup_WrongType_Httpsqs(t *testing.T) {
	msg := NewHttpsqSMessage("q", nil, 1)
	_, err := msg.KafkaGroup()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "KafkaGroup()")
	assert.Contains(t, err.Error(), "httpsqs")
}

func TestMessage_HttpsqSPosition_WrongType(t *testing.T) {
	msg := NewRedisMessage("q", nil)
	_, err := msg.HttpsqSPosition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HttpsqSPosition()")
	assert.Contains(t, err.Error(), "redis")
}

func TestMessage_HttpsqSPosition_WrongType_Kafka(t *testing.T) {
	msg := NewKafkaMessage("g", "t", nil)
	_, err := msg.HttpsqSPosition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HttpsqSPosition()")
	assert.Contains(t, err.Error(), "kafka")
}

// --- 类型化访问器：在正确的 MQ 类型上调用时成功 ---

func TestMessage_KafkaGroup_Success(t *testing.T) {
	msg := NewKafkaMessage("my-group", "my-topic", []byte("data"))
	group, err := msg.KafkaGroup()
	assert.NoError(t, err)
	assert.Equal(t, "my-group", group)
}

func TestMessage_HttpsqSPosition_Success(t *testing.T) {
	msg := NewHttpsqSMessage("q", []byte("data"), 99)
	pos, err := msg.HttpsqSPosition()
	assert.NoError(t, err)
	assert.Equal(t, int64(99), pos)
}

// --- mqSystem.String() 测试 ---

func TestMqSystem_String(t *testing.T) {
	assert.Equal(t, "redis", systemRedis.String())
	assert.Equal(t, "kafka", systemKafka.String())
	assert.Equal(t, "httpsqs", systemHttpsqs.String())
}

// --- Validate 测试 ---

func TestMessage_Validate_NonStrict_AlwaysNil(t *testing.T) {
	// 非严格模式下，即使字段不一致也返回 nil
	strictMode = false
	defer func() { strictMode = false }()

	// redis 消息但有 Group（不一致）
	msg := NewRedisMessage("q", nil)
	msg.Group = "should-not-be-here"
	assert.NoError(t, msg.Validate())
}

func TestMessage_Validate_Strict_RedisValid(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewRedisMessage("q", []byte("data"))
	assert.NoError(t, msg.Validate())
}

func TestMessage_Validate_Strict_RedisInvalid_GroupNotEmpty(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewRedisMessage("q", []byte("data"))
	msg.Group = "unexpected-group"
	err := msg.Validate()
	assert.Error(t, err)
}

func TestMessage_Validate_Strict_RedisInvalid_PosNonZero(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewRedisMessage("q", []byte("data"))
	msg.Pos = 10
	err := msg.Validate()
	assert.Error(t, err)
}

func TestMessage_Validate_Strict_KafkaValid(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewKafkaMessage("my-group", "my-topic", []byte("data"))
	assert.NoError(t, msg.Validate())
}

func TestMessage_Validate_Strict_KafkaInvalid_PosNonZero(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewKafkaMessage("my-group", "my-topic", []byte("data"))
	msg.Pos = 5
	err := msg.Validate()
	assert.Error(t, err)
}

func TestMessage_Validate_Strict_KafkaInvalid_GroupEmpty(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	// 构造一个 Group 为空的 kafka 消息
	msg := NewKafkaMessage("", "my-topic", []byte("data"))
	err := msg.Validate()
	assert.Error(t, err)
}

func TestMessage_Validate_Strict_HttpsqsValid(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewHttpsqSMessage("q", []byte("data"), 1)
	assert.NoError(t, msg.Validate())
}

func TestMessage_Validate_Strict_HttpsqsInvalid_GroupNotEmpty(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewHttpsqSMessage("q", []byte("data"), 1)
	msg.Group = "unexpected-group"
	err := msg.Validate()
	assert.Error(t, err)
}

func TestMessage_Validate_Strict_HttpsqsInvalid_PosZero(t *testing.T) {
	strictMode = true
	defer func() { strictMode = false }()

	msg := NewHttpsqSMessage("q", []byte("data"), 0)
	err := msg.Validate()
	assert.Error(t, err)
}

// --- 额外边界测试 ---

func TestMessage_DataNil(t *testing.T) {
	msg := NewRedisMessage("q", nil)
	assert.Nil(t, msg.Data)
}

func TestMessage_EmptyData(t *testing.T) {
	msg := NewRedisMessage("q", []byte{})
	assert.Empty(t, msg.Data)
}

func TestMessage_KafkaGroup_OnHttpsqs_ReturnsHttpsqsInError(t *testing.T) {
	msg := NewHttpsqSMessage("q", nil, 1)
	_, err := msg.KafkaGroup()
	assert.True(t, errors.Is(err, ErrWrongMQType))
}

func TestMessage_HttpsqSPosition_OnKafka_ReturnsError(t *testing.T) {
	msg := NewKafkaMessage("g", "t", nil)
	_, err := msg.HttpsqSPosition()
	assert.True(t, errors.Is(err, ErrWrongMQType))
}

// --- RetryMode 常量测试 ---

func TestRetryModeConstants(t *testing.T) {
	assert.Equal(t, RetryMode(0), RetryModeSync)
	assert.Equal(t, RetryMode(1), RetryModeRequeue)
}

// --- SetStrictMode ---

func TestSetStrictMode_EnableAndDisable(t *testing.T) {
	// 确认默认关闭
	assert.False(t, strictMode)

	// 启用严格模式
	SetStrictMode(true)
	assert.True(t, strictMode)

	// 在严格模式下 Validate 执行检查
	msg := NewRedisMessage("q", nil)
	msg.Group = "bad"
	assert.Error(t, msg.Validate())

	// 禁用严格模式
	SetStrictMode(false)
	assert.False(t, strictMode)

	// 禁用后 Validate 返回 nil
	assert.NoError(t, msg.Validate())
}

// --- mqSystem.String default 分支 ---

func TestMqSystem_String_Unknown(t *testing.T) {
	// 覆盖 default 分支：未知系统类型返回 "unknown"
	unknown := mqSystem(99)
	assert.Equal(t, "unknown", unknown.String())
}
