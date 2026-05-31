package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConsumerMetrics(t *testing.T) {
	m := NewConsumerMetrics()
	assert.NotNil(t, m)
}

func TestConsumerMetrics_NilSafety(t *testing.T) {
	var m *ConsumerMetrics
	assert.NotPanics(t, func() { m.OnConsume() })
	assert.NotPanics(t, func() { m.OnRetry() })
	assert.NotPanics(t, func() { m.OnDeadLetter() })
}

func TestConsumerMetrics_Methods(t *testing.T) {
	m := NewConsumerMetrics()
	assert.NotPanics(t, func() { m.OnConsume() })
	assert.NotPanics(t, func() { m.OnRetry() })
	assert.NotPanics(t, func() { m.OnDeadLetter() })
}
