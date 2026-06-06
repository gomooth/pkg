package types

// RetryMode 重试模式
type RetryMode int

const (
	// RetryModeSync 同步阻塞重试
	RetryModeSync RetryMode = iota
	// RetryModeRequeue 再入队重试
	RetryModeRequeue
)
