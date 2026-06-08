package kafka

import (
	"context"
	"time"

	"github.com/gomooth/pkg/mq/internal/types"
)

// ==================== Consumer ====================

// IHandler 消息处理器接口
type IHandler = types.IHandler

// DeadLetterHandler 可选死信接口，重试耗尽后调用。
type DeadLetterHandler = types.DeadLetterHandler

// FuncHandler 函数适配器，将函数转换为 IHandler
type FuncHandler = types.FuncHandler

// IConsumeServer 消费者服务接口
type IConsumeServer = types.IConsumeServer

// ConsumerRegistration 消费者注册信息
type ConsumerRegistration struct {
	Group   string
	Handler IHandler
	Topics  []string
}

// ==================== Producer ====================

// IProducer 生产者接口（集成 app.IApp 生命周期）
type IProducer = types.IProducer

// ==================== 重试模式 ====================

// RetryMode 重试模式
type RetryMode = types.RetryMode

const (
	// RetryModeSync 同步阻塞重试
	RetryModeSync = types.RetryModeSync
	// RetryModeAsync 异步非阻塞重试（存储后端由 RetryStore 实现决定：
	// MemoryRetryStore 为水位线模式，RedisRetryStore 为 Redis 持久化模式）
	RetryModeAsync = types.RetryModeRequeue // kafka uses Async instead of Requeue
)

// ==================== 失败处理器 ====================

// FailedHandlerFunc 失败处理回调函数类型
type FailedHandlerFunc = types.FailedHandlerFunc

// ==================== 统一消息类型 ====================

// Message 统一消息类型
type Message = types.Message

// ==================== 注册/生产选项 ====================

// RegisterOption 注册消费者时的配置选项
type RegisterOption = types.RegisterOption

// ProduceOption 生产消息时的配置选项
type ProduceOption = types.ProduceOption

// ==================== 重试存储 ====================

// RetryStore 异步重试的存储后端接口
type RetryStore interface {
	Schedule(ctx context.Context, item *RetryItem) error
	Fetch(ctx context.Context, now time.Time, limit int) ([]*RetryItem, error)
	Remove(ctx context.Context, item *RetryItem) error
	Reschedule(ctx context.Context, oldItem, newItem *RetryItem) error
	LoadAll(ctx context.Context) ([]*RetryItem, error)
	Close() error
}

// WatermarkStore 水位线存储扩展接口
type WatermarkStore interface {
	RetryStore
	MarkSuccess(topic string, partition int32, offset int64)
	RemovePending(topic string, partition int32, offset int64)
	Watermark(topic string, partition int32) (int64, bool)
	ResetPartition(topic string, partition int32)
	Notify() chan struct{}
}

// RetryItem 待重试消息的完整表示（公开类型）
type RetryItem struct {
	Topic         string
	Partition     int32
	Offset        int64
	Key           []byte
	Value         []byte
	Headers       []HeaderKV
	Attempt       int
	NextRetryAt   time.Time
	ConsumerGroup string
}

// HeaderKV 消息头键值对（公开类型，Key 为 string 便于 JSON 序列化）
type HeaderKV struct {
	Key   string
	Value []byte
}