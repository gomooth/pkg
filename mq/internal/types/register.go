package types

// RegisterOption 注册消费者时的配置选项
type RegisterOption func(*RegisterConfig)

// RegisterConfig 注册配置（导出，供各 MQ 实现在 Register 中解析选项）
type RegisterConfig struct {
	Group       string        // kafka 专有：consumer group
	ExtraTopics []string      // kafka 专有：额外 topic
	QueueOpts   []QueueOption // httpsqs 专有：队列级别配置
}

// ApplyRegisterOptions 应用选项并返回解析后的配置
func ApplyRegisterOptions(opts []RegisterOption) *RegisterConfig {
	cfg := &RegisterConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithGroup 设置 Kafka consumer group。
func WithGroup(group string) RegisterOption {
	return func(c *RegisterConfig) { c.Group = group }
}

// WithExtraTopics 设置 Kafka 额外消费的 topic。
func WithExtraTopics(topics ...string) RegisterOption {
	return func(c *RegisterConfig) { c.ExtraTopics = append(c.ExtraTopics, topics...) }
}

// WithQueueOptions 设置 HTTPSQS 队列级别配置。
func WithQueueOptions(opts ...QueueOption) RegisterOption {
	return func(c *RegisterConfig) { c.QueueOpts = append(c.QueueOpts, opts...) }
}

// QueueOption HTTPSQS 单队列级别配置选项
type QueueOption func(*QueueConfig)

// QueueConfig 队列级别配置
type QueueConfig struct {
	Client    any           // httpsqs.IClient（使用 any 避免循环导入）
	MaxRetry  *int
	Backoff   any           // retry.BackoffStrategy（使用 any 避免循环导入）
	RetryMode *RetryMode
	FailedFn  FailedHandlerFunc
}

// WithQueueClient 设置队列级别的 HTTPSQS 客户端
func WithQueueClient(client any) QueueOption {
	return func(c *QueueConfig) { c.Client = client }
}

// WithQueueMaxRetry 设置队列级别的最大重试次数
func WithQueueMaxRetry(n int) QueueOption {
	return func(c *QueueConfig) { c.MaxRetry = &n }
}

// WithQueueBackoff 设置队列级别的退避策略
func WithQueueBackoff(backoff any) QueueOption {
	return func(c *QueueConfig) { c.Backoff = backoff }
}

// WithQueueRetryMode 设置队列级别的重试模式
func WithQueueRetryMode(mode RetryMode) QueueOption {
	return func(c *QueueConfig) { c.RetryMode = &mode }
}

// WithQueueFailedHandler 设置队列级别的失败处理器
func WithQueueFailedHandler(fn FailedHandlerFunc) QueueOption {
	return func(c *QueueConfig) { c.FailedFn = fn }
}
