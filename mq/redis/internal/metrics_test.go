package internal

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestNewConsumerMetrics(t *testing.T) {
	cm := NewConsumerMetrics()
	assert.NotNil(t, cm)
	assert.NotPanics(t, func() { cm.OnConsume() })
	assert.NotPanics(t, func() { cm.OnRetry() })
	assert.NotPanics(t, func() { cm.OnDeadLetter() })
}

func TestConsumerMetrics_Nil(t *testing.T) {
	var cm *ConsumerMetrics
	assert.NotPanics(t, func() { cm.OnConsume() })
	assert.NotPanics(t, func() { cm.OnRetry() })
	assert.NotPanics(t, func() { cm.OnDeadLetter() })
}

func TestNewProducerMetrics(t *testing.T) {
	pm := NewProducerMetrics()
	assert.NotNil(t, pm)
	assert.NotPanics(t, func() { pm.OnProduce(1) })
	assert.NotPanics(t, func() { pm.OnProduce(5) })
	assert.NotPanics(t, func() { pm.OnError() })
}

func TestProducerMetrics_Nil(t *testing.T) {
	var pm *ProducerMetrics
	assert.NotPanics(t, func() { pm.OnProduce(1) })
	assert.NotPanics(t, func() { pm.OnError() })
}

func TestBuildConsumerOptions(t *testing.T) {
	opts := BuildConsumerOptions("localhost:6379")
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Equal(t, 0, int(opts.ReadTimeout))
	assert.Greater(t, opts.PoolSize, 0)
}

func TestBuildProducerOptions(t *testing.T) {
	opts := BuildProducerOptions("localhost:6379")
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Equal(t, 0, int(opts.ReadTimeout))
	assert.Greater(t, opts.PoolSize, 0)
}

func TestBuildConsumerOptions_WithOverrides(t *testing.T) {
	opts := BuildConsumerOptions("localhost:6379", func(o *redis.Options) {
		o.DB = 5
	})
	assert.Equal(t, 5, opts.DB)
}

func TestBuildProducerOptions_WithOverrides(t *testing.T) {
	opts := BuildProducerOptions("localhost:6379", func(o *redis.Options) {
		o.DB = 3
	})
	assert.Equal(t, 3, opts.DB)
}
