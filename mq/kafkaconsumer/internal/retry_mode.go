package internal

// RetryMode 定义重试模式
type RetryMode int

const (
	// RetryModeSync 同步阻塞重试（默认），消息处理失败时在消费协程内同步重试，阻塞 partition
	RetryModeSync RetryMode = iota
	// RetryModeAsyncWatermark 异步重试 + 水位线 offset 跟踪，不依赖外部存储，重启后由 Kafka 重投递未提交的消息
	RetryModeAsyncWatermark
	// RetryModeAsyncRedis 异步重试 + Redis 持久化，失败消息持久化到 Redis 后立即提交 offset，重启后从 Redis 恢复
	RetryModeAsyncRedis
)
