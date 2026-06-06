package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConsumerMetrics(t *testing.T) {
	m := NewConsumerMetrics("test")
	assert.NotNil(t, m)
}

func TestConsumerMetrics_OnConsume(t *testing.T) {
	m := NewConsumerMetrics("test")
	assert.NotPanics(t, func() {
		m.OnConsume()
	})
}

func TestConsumerMetrics_OnRetry(t *testing.T) {
	m := NewConsumerMetrics("test")
	assert.NotPanics(t, func() {
		m.OnRetry()
	})
}

func TestConsumerMetrics_OnDeadLetter(t *testing.T) {
	m := NewConsumerMetrics("test")
	assert.NotPanics(t, func() {
		m.OnDeadLetter()
	})
}

func TestConsumerMetrics_NilReceiver(t *testing.T) {
	var m *ConsumerMetrics
	assert.NotPanics(t, func() {
		m.OnConsume()
		m.OnRetry()
		m.OnDeadLetter()
	})
}
