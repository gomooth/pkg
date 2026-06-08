package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ApplyProduceOptions ---

func TestApplyProduceOptions_Empty(t *testing.T) {
	// 无选项时返回零值配置
	cfg := ApplyProduceOptions(nil)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.OrderKey)
}

func TestApplyProduceOptions_WithOrderKey(t *testing.T) {
	cfg := ApplyProduceOptions([]ProduceOption{WithOrderKey("order-1")})
	assert.Equal(t, "order-1", cfg.OrderKey)
}

func TestApplyProduceOptions_MultipleOptions(t *testing.T) {
	// 后一个 WithOrderKey 覆盖前一个
	cfg := ApplyProduceOptions([]ProduceOption{
		WithOrderKey("first"),
		WithOrderKey("second"),
	})
	assert.Equal(t, "second", cfg.OrderKey)
}

// --- WithOrderKey ---

func TestWithOrderKey_SetsField(t *testing.T) {
	cfg := &ProduceConfig{}
	WithOrderKey("my-key")(cfg)
	assert.Equal(t, "my-key", cfg.OrderKey)
}

func TestWithOrderKey_EmptyKey(t *testing.T) {
	cfg := &ProduceConfig{}
	WithOrderKey("")(cfg)
	assert.Empty(t, cfg.OrderKey)
}
