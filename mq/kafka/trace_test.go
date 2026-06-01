package kafka

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
)

func TestKafkaMessageCarrier_Get(t *testing.T) {
	headers := []*sarama.RecordHeader{
		{Key: []byte("trace-id"), Value: []byte("123")},
		{Key: []byte("span-id"), Value: []byte("456")},
	}
	carrier := newKafkaMessageCarrier(headers)

	assert.Equal(t, "123", carrier.Get("trace-id"))
	assert.Equal(t, "456", carrier.Get("span-id"))
	assert.Equal(t, "", carrier.Get("missing"))
}

func TestKafkaMessageCarrier_Set(t *testing.T) {
	headers := []*sarama.RecordHeader{
		{Key: []byte("existing"), Value: []byte("value1")},
	}
	carrier := newKafkaMessageCarrier(headers)

	// Add new header
	carrier.Set("new-key", "new-value")
	assert.Equal(t, "new-value", carrier.Get("new-key"))
	assert.Equal(t, "value1", carrier.Get("existing"))

	// Overwrite existing header
	carrier.Set("existing", "updated")
	assert.Equal(t, "updated", carrier.Get("existing"))
}

func TestKafkaMessageCarrier_Keys(t *testing.T) {
	headers := []*sarama.RecordHeader{
		{Key: []byte("a"), Value: []byte("1")},
		{Key: []byte("b"), Value: []byte("2")},
	}
	carrier := newKafkaMessageCarrier(headers)

	keys := carrier.Keys()
	assert.ElementsMatch(t, []string{"a", "b"}, keys)
}

func TestKafkaMessageCarrier_EmptyHeaders(t *testing.T) {
	carrier := newKafkaMessageCarrier(nil)
	assert.Equal(t, "", carrier.Get("any"))
	assert.Empty(t, carrier.Keys())

	carrier.Set("new", "val")
	assert.Equal(t, "val", carrier.Get("new"))
}

func TestKafkaProducerCarrier_Get(t *testing.T) {
	msg := &sarama.ProducerMessage{
		Headers: []sarama.RecordHeader{
			{Key: []byte("trace-id"), Value: []byte("abc")},
		},
	}
	carrier := &kafkaProducerCarrier{msg: msg}

	assert.Equal(t, "abc", carrier.Get("trace-id"))
	assert.Equal(t, "", carrier.Get("missing"))
}

func TestKafkaProducerCarrier_Set(t *testing.T) {
	msg := &sarama.ProducerMessage{}
	carrier := &kafkaProducerCarrier{msg: msg}

	carrier.Set("key1", "value1")
	carrier.Set("key2", "value2")

	assert.Equal(t, "value1", carrier.Get("key1"))
	assert.Equal(t, "value2", carrier.Get("key2"))
}

func TestKafkaProducerCarrier_Keys(t *testing.T) {
	msg := &sarama.ProducerMessage{
		Headers: []sarama.RecordHeader{
			{Key: []byte("k1"), Value: []byte("v1")},
			{Key: []byte("k2"), Value: []byte("v2")},
		},
	}
	carrier := &kafkaProducerCarrier{msg: msg}

	keys := carrier.Keys()
	assert.ElementsMatch(t, []string{"k1", "k2"}, keys)
}

func TestKafkaProducerCarrier_EmptyHeaders(t *testing.T) {
	msg := &sarama.ProducerMessage{}
	carrier := &kafkaProducerCarrier{msg: msg}

	assert.Equal(t, "", carrier.Get("any"))
	assert.Empty(t, carrier.Keys())

	carrier.Set("new", "val")
	assert.Equal(t, "val", carrier.Get("new"))
}

func TestPropagateFromMessage(t *testing.T) {
	// Test with no trace headers - should return context unchanged
	msg := &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{},
	}
	ctx := context.Background()
	resultCtx := propagateFromMessage(ctx, msg)
	assert.NotNil(t, resultCtx)
}

func TestStartConsumerSpan(t *testing.T) {
	msg := &sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    100,
		Headers:   []*sarama.RecordHeader{},
	}
	ctx := context.Background()
	resultCtx, span := startConsumerSpan(ctx, msg)
	defer span.End()

	assert.NotNil(t, resultCtx)
	assert.NotNil(t, span)
}

func TestInjectProducerTrace(t *testing.T) {
	msgs := []*sarama.ProducerMessage{
		{Topic: "test-topic", Value: sarama.StringEncoder("hello")},
		{Topic: "test-topic", Value: sarama.StringEncoder("world")},
	}
	ctx := context.Background()
	resultCtx, span := injectProducerTrace(ctx, "test-topic", msgs)
	defer span.End()

	assert.NotNil(t, resultCtx)
	assert.NotNil(t, span)
	// Headers may or may not be injected depending on OTel configuration
}

func TestInjectProducerTrace_EmptyMessages(t *testing.T) {
	msgs := []*sarama.ProducerMessage{}
	ctx := context.Background()
	resultCtx, span := injectProducerTrace(ctx, "test-topic", msgs)
	defer span.End()

	assert.NotNil(t, resultCtx)
	assert.NotNil(t, span)
}
