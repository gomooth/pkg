package internal

import (
	"testing"
	"time"
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
