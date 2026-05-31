package internal

import "testing"

func TestNewConsumerMetrics(t *testing.T) {
	m := NewConsumerMetrics()
	if m == nil {
		t.Fatal("expected non-nil ConsumerMetrics")
	}
	if m.consumeCounter == nil {
		t.Error("expected consumeCounter to be initialized")
	}
	if m.retryCounter == nil {
		t.Error("expected retryCounter to be initialized")
	}
	if m.deadLetterCounter == nil {
		t.Error("expected deadLetterCounter to be initialized")
	}
}

func TestNewProducerMetrics(t *testing.T) {
	m := NewProducerMetrics()
	if m == nil {
		t.Fatal("expected non-nil ProducerMetrics")
	}
	if m.produceCounter == nil {
		t.Error("expected produceCounter to be initialized")
	}
	if m.errorCounter == nil {
		t.Error("expected errorCounter to be initialized")
	}
}
