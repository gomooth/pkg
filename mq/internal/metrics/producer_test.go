package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProducerMetrics(t *testing.T) {
	m := NewProducerMetrics("test")
	assert.NotNil(t, m)
}

func TestProducerMetrics_OnProduce(t *testing.T) {
	m := NewProducerMetrics("test")
	assert.NotPanics(t, func() {
		m.OnProduce(1)
		m.OnProduce(10)
	})
}

func TestProducerMetrics_OnError(t *testing.T) {
	m := NewProducerMetrics("test")
	assert.NotPanics(t, func() {
		m.OnError()
	})
}

func TestProducerMetrics_NilReceiver(t *testing.T) {
	var m *ProducerMetrics
	assert.NotPanics(t, func() {
		m.OnProduce(1)
		m.OnError()
	})
}
