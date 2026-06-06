package httpsqs

import (
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
	Queue   string
	Handler IHandler
	Opts    []QueueOption
}

// ==================== 重试模式 ====================

// RetryMode 重试模式
type RetryMode = types.RetryMode

const (
	// RetryModeSync 同步阻塞重试：Handle 失败后在当前 goroutine 中立即重试
	RetryModeSync = types.RetryModeSync
	// RetryModeRequeue 再入队重试：Handle 失败后通过 HTTPSQS Put 将消息放回队列尾部
	RetryModeRequeue = types.RetryModeRequeue
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

// QueueOption 单队列级别配置（覆盖全局默认值）
type QueueOption = types.QueueOption