package mq

import "github.com/gomooth/pkg/mq/internal/types"

// Message 统一消息类型（re-export from internal/types）
type Message = types.Message

// ErrWrongMQType 表示在错误的 MQ 类型消息上调用了类型专属访问器。
var ErrWrongMQType = types.ErrWrongMQType

// SetStrictMode 启用或禁用严格验证模式。
func SetStrictMode(enabled bool) { types.SetStrictMode(enabled) }

// 消息构造函数
func NewRedisMessage(queue string, data []byte) Message {
	return types.NewRedisMessage(queue, data)
}

func NewKafkaMessage(group, topic string, data []byte) Message {
	return types.NewKafkaMessage(group, topic, data)
}

func NewHttpsqSMessage(queue string, data []byte, pos int64) Message {
	return types.NewHttpsqSMessage(queue, data, pos)
}
