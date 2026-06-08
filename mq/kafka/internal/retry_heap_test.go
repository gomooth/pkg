package internal

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryHeap_Order(t *testing.T) {
	h := NewRetryHeap(0)

	now := time.Now()
	h.PushItem(&RetryItem{Offset: 3, NextRetryAt: now.Add(3 * time.Second)})
	h.PushItem(&RetryItem{Offset: 1, NextRetryAt: now.Add(1 * time.Second)})
	h.PushItem(&RetryItem{Offset: 2, NextRetryAt: now.Add(2 * time.Second)})

	item := h.PopItem()
	if item.Offset != 1 {
		t.Errorf("expected offset 1 (earliest), got %d", item.Offset)
	}
	item = h.PopItem()
	if item.Offset != 2 {
		t.Errorf("expected offset 2, got %d", item.Offset)
	}
}

func TestRetryHeap_IsFull(t *testing.T) {
	h := NewRetryHeap(2)

	now := time.Now()
	h.PushItem(&RetryItem{Offset: 1, NextRetryAt: now})
	if h.IsFull() {
		t.Error("expected not full after 1 item")
	}

	h.PushItem(&RetryItem{Offset: 2, NextRetryAt: now})
	if !h.IsFull() {
		t.Error("expected full after 2 items with maxQueueSize=2")
	}
}

func TestRetryHeap_Peek(t *testing.T) {
	h := NewRetryHeap(0)
	if h.Peek() != nil {
		t.Error("expected nil peek on empty heap")
	}

	now := time.Now()
	h.PushItem(&RetryItem{Offset: 5, NextRetryAt: now.Add(5 * time.Second)})
	h.PushItem(&RetryItem{Offset: 1, NextRetryAt: now.Add(1 * time.Second)})

	item := h.Peek()
	if item.Offset != 1 {
		t.Errorf("expected peek offset 1, got %d", item.Offset)
	}
	if h.Len() != 2 {
		t.Error("peek should not remove item")
	}
}

func TestRetryHeap_Remove(t *testing.T) {
	now := time.Now()

	t.Run("remove existing item returns true", func(t *testing.T) {
		h := NewRetryHeap(0)
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 1, NextRetryAt: now.Add(1 * time.Second)})
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 2, NextRetryAt: now.Add(2 * time.Second)})
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 3, NextRetryAt: now.Add(3 * time.Second)})

		result := h.Remove("t", 0, 2)
		assert.True(t, result, "Remove should return true for existing item")
		assert.Equal(t, 2, h.Len(), "heap should have 2 items after remove")

		// Verify heap order is still correct
		item := h.PopItem()
		assert.Equal(t, int64(1), item.Offset)
		item = h.PopItem()
		assert.Equal(t, int64(3), item.Offset)
	})

	t.Run("remove non-existing item returns false", func(t *testing.T) {
		h := NewRetryHeap(0)
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 1, NextRetryAt: now.Add(1 * time.Second)})

		result := h.Remove("t", 0, 999)
		assert.False(t, result, "Remove should return false for non-existing item")
		assert.Equal(t, 1, h.Len(), "heap size should not change")
	})

	t.Run("remove top element", func(t *testing.T) {
		h := NewRetryHeap(0)
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 1, NextRetryAt: now.Add(1 * time.Second)})
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 2, NextRetryAt: now.Add(2 * time.Second)})

		result := h.Remove("t", 0, 1)
		assert.True(t, result)
		assert.Equal(t, 1, h.Len())

		// Next pop should return offset 2 (former second item)
		item := h.PopItem()
		assert.Equal(t, int64(2), item.Offset)
	})

	t.Run("remove middle element", func(t *testing.T) {
		h := NewRetryHeap(0)
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 1, NextRetryAt: now.Add(1 * time.Second)})
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 2, NextRetryAt: now.Add(2 * time.Second)})
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 3, NextRetryAt: now.Add(3 * time.Second)})
		h.PushItem(&RetryItem{Topic: "t", Partition: 0, Offset: 4, NextRetryAt: now.Add(4 * time.Second)})

		result := h.Remove("t", 0, 2)
		assert.True(t, result)
		assert.Equal(t, 3, h.Len())

		// Verify remaining items pop in correct order
		var offsets []int64
		for h.Len() > 0 {
			offsets = append(offsets, h.PopItem().Offset)
		}
		assert.Equal(t, []int64{1, 3, 4}, offsets)
	})
}

func TestRetryHeap_PopItem_Empty(t *testing.T) {
	h := NewRetryHeap(0)
	item := h.PopItem()
	assert.Nil(t, item, "PopItem on empty heap should return nil")
}

func TestSaramaMsgToRetryItem(t *testing.T) {
	now := time.Now()

	t.Run("normal message with headers", func(t *testing.T) {
		msg := &sarama.ConsumerMessage{
			Topic:     "test-topic",
			Partition: 1,
			Offset:    100,
			Key:       []byte("key1"),
			Value:     []byte("value1"),
			Headers: []*sarama.RecordHeader{
				{Key: []byte("h1"), Value: []byte("v1")},
				{Key: []byte("h2"), Value: []byte("v2")},
			},
		}

		item := SaramaMsgToRetryItem(msg, "my-group", 2, now)

		assert.Equal(t, "test-topic", item.Topic)
		assert.Equal(t, int32(1), item.Partition)
		assert.Equal(t, int64(100), item.Offset)
		assert.Equal(t, []byte("key1"), item.Key)
		assert.Equal(t, []byte("value1"), item.Value)
		assert.Equal(t, uint(2), item.Attempt)
		assert.Equal(t, now, item.NextRetryAt)
		assert.Equal(t, "my-group", item.ConsumerGroup)
		require.Len(t, item.Headers, 2)
		assert.Equal(t, []byte("h1"), item.Headers[0].Key)
		assert.Equal(t, []byte("v1"), item.Headers[0].Value)
		assert.Equal(t, []byte("h2"), item.Headers[1].Key)
		assert.Equal(t, []byte("v2"), item.Headers[1].Value)
	})

	t.Run("message with empty headers", func(t *testing.T) {
		msg := &sarama.ConsumerMessage{
			Topic:     "test-topic",
			Partition: 0,
			Offset:    50,
			Headers:   []*sarama.RecordHeader{},
		}

		item := SaramaMsgToRetryItem(msg, "group", 0, now)
		assert.Empty(t, item.Headers, "headers should be empty but not nil")
	})

	t.Run("message with nil headers", func(t *testing.T) {
		msg := &sarama.ConsumerMessage{
			Topic:     "test-topic",
			Partition: 0,
			Offset:    51,
			Headers:   nil,
		}

		item := SaramaMsgToRetryItem(msg, "group", 0, now)
		assert.Empty(t, item.Headers)
	})

	t.Run("multiple headers all converted", func(t *testing.T) {
		msg := &sarama.ConsumerMessage{
			Topic:     "test-topic",
			Partition: 0,
			Offset:    52,
			Headers: []*sarama.RecordHeader{
				{Key: []byte("a"), Value: []byte("1")},
				{Key: []byte("b"), Value: []byte("2")},
				{Key: []byte("c"), Value: []byte("3")},
			},
		}

		item := SaramaMsgToRetryItem(msg, "group", 1, now)
		require.Len(t, item.Headers, 3)
		assert.Equal(t, []byte("a"), item.Headers[0].Key)
		assert.Equal(t, []byte("b"), item.Headers[1].Key)
		assert.Equal(t, []byte("c"), item.Headers[2].Key)
	})
}
