// Package types 定义 MQ 子系统共享的统一消息类型。
// 外部包必须通过构造函数（NewRedisMessage / NewKafkaMessage / NewHttpsqSMessage）创建 Message，
// 以保证 MQ 类型与字段的一致性。
package types

import (
	"errors"
	"fmt"
)

// mqSystem 标识消息所属的 MQ 子系统。
type mqSystem int

const (
	systemRedis   mqSystem = iota // Redis 队列
	systemKafka                   // Kafka
	systemHttpsqs                 // HTTPSQS
)

// String 返回 MQ 子系统的可读名称。
func (s mqSystem) String() string {
	switch s {
	case systemRedis:
		return "redis"
	case systemKafka:
		return "kafka"
	case systemHttpsqs:
		return "httpsqs"
	default:
		return "unknown"
	}
}

// ErrWrongMQType 表示在错误的 MQ 类型消息上调用了类型专属访问器。
var ErrWrongMQType = errors.New("mq: accessor called on wrong message type")

// Message 是跨 MQ 子系统的统一消息类型。
// 私有 system 字段阻止外部直接构造不一致的组合；
// 必须使用 NewRedisMessage / NewKafkaMessage / NewHttpsqSMessage 构造。
type Message struct {
	system mqSystem // 私有：阻止外部构造无效组合
	Queue  string   // 目标名称（redis/httpsqs=队列名，kafka=topic）
	Data   []byte   // 消息体
	Group  string   // Kafka 专属：消费者组
	Pos    int64    // HTTPSQS 专属：队列位置
}

// NewRedisMessage 创建 Redis 队列消息。
func NewRedisMessage(queue string, data []byte) Message {
	return Message{
		system: systemRedis,
		Queue:  queue,
		Data:   data,
	}
}

// NewKafkaMessage 创建 Kafka 消息。
func NewKafkaMessage(group, topic string, data []byte) Message {
	return Message{
		system: systemKafka,
		Queue:  topic,
		Data:   data,
		Group:  group,
	}
}

// NewHttpsqSMessage 创建 HTTPSQS 消息。
func NewHttpsqSMessage(queue string, data []byte, pos int64) Message {
	return Message{
		system: systemHttpsqs,
		Queue:  queue,
		Data:   data,
		Pos:    pos,
	}
}

// IsRedis 报告消息是否来自 Redis 队列。
func (m Message) IsRedis() bool { return m.system == systemRedis }

// IsKafka 报告消息是否来自 Kafka。
func (m Message) IsKafka() bool { return m.system == systemKafka }

// IsHttpsqs 报告消息是否来自 HTTPSQS。
func (m Message) IsHttpsqs() bool { return m.system == systemHttpsqs }

// KafkaGroup 返回 Kafka 消费者组。
// 若消息不是 Kafka 类型则返回错误。
func (m Message) KafkaGroup() (string, error) {
	if !m.IsKafka() {
		return "", fmt.Errorf("mq: KafkaGroup() called on %s message: %w", m.system, ErrWrongMQType)
	}
	return m.Group, nil
}

// HttpsqSPosition 返回 HTTPSQS 队列位置。
// 若消息不是 HTTPSQS 类型则返回错误。
func (m Message) HttpsqSPosition() (int64, error) {
	if !m.IsHttpsqs() {
		return 0, fmt.Errorf("mq: HttpsqSPosition() called on %s message: %w", m.system, ErrWrongMQType)
	}
	return m.Pos, nil
}

// strictMode 控制 Validate 是否执行严格字段一致性检查。默认关闭。
var strictMode bool

// SetStrictMode 启用或禁用严格验证模式。
func SetStrictMode(enabled bool) { strictMode = enabled }

// Validate 检查消息的字段一致性。
// 严格模式下会验证 MQ 专属字段只在对应的类型上使用：
//   - Redis: Group 须为空，Pos 须为零
//   - Kafka: Group 须非空，Pos 须为零
//   - HTTPSQS: Group 须为空，Pos 须非零
//
// 非严格模式始终返回 nil。
func (m Message) Validate() error {
	if !strictMode {
		return nil
	}

	switch m.system {
	case systemRedis:
		if m.Group != "" {
			return fmt.Errorf("mq: redis message must not have Group, got %q", m.Group)
		}
		if m.Pos != 0 {
			return fmt.Errorf("mq: redis message must not have Pos, got %d", m.Pos)
		}
	case systemKafka:
		if m.Group == "" {
			return fmt.Errorf("mq: kafka message must have non-empty Group")
		}
		if m.Pos != 0 {
			return fmt.Errorf("mq: kafka message must not have Pos, got %d", m.Pos)
		}
	case systemHttpsqs:
		if m.Group != "" {
			return fmt.Errorf("mq: httpsqs message must not have Group, got %q", m.Group)
		}
		if m.Pos == 0 {
			return fmt.Errorf("mq: httpsqs message must have non-zero Pos")
		}
	}
	return nil
}
