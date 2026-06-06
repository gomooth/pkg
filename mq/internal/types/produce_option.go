package types

// ProduceOption 生产消息时的配置选项
type ProduceOption func(*ProduceConfig)

// ProduceConfig 生产配置
type ProduceConfig struct {
	OrderKey string // kafka 专有：有序生产的分区键
}

// ApplyProduceOptions 应用选项并返回解析后的配置
func ApplyProduceOptions(opts []ProduceOption) *ProduceConfig {
	cfg := &ProduceConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithOrderKey 设置 Kafka 有序生产的分区键。
// 原 kafka.IProducer.ProduceOrdered 合并为此选项。
// 非 Kafka 实现收到此选项时忽略。
func WithOrderKey(key string) ProduceOption {
	return func(c *ProduceConfig) { c.OrderKey = key }
}
