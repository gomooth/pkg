package kafka

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRetryStore_ScheduleAndFetch(t *testing.T) {
	store := NewMemoryRetryStore()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-1 * time.Second),
	}
	if err := store.Schedule(context.Background(), item); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	items, err := store.Fetch(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Offset != 1 {
		t.Errorf("expected offset 1, got %d", items[0].Offset)
	}
}

func TestMemoryRetryStore_FetchNotDue(t *testing.T) {
	store := NewMemoryRetryStore()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1,
		NextRetryAt: now.Add(10 * time.Second),
	}
	store.Schedule(context.Background(), item)
	items, err := store.Fetch(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestMemoryRetryStore_WatermarkTracking(t *testing.T) {
	store := NewMemoryRetryStore()
	store.Schedule(context.Background(), &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, NextRetryAt: time.Now().Add(-time.Second),
	})
	store.Schedule(context.Background(), &RetryItem{
		Topic: "test", Partition: 0, Offset: 3, NextRetryAt: time.Now().Add(-time.Second),
	})
	store.MarkSuccess("test", 0, 2)
	wm, ok := store.Watermark("test", 0)
	if !ok {
		t.Fatal("expected watermark to exist")
	}
	if wm != 0 {
		t.Errorf("expected watermark 0 (blocked by pending 1), got %d", wm)
	}
	store.MarkSuccess("test", 0, 1)
	wm, ok = store.Watermark("test", 0)
	if !ok {
		t.Fatal("expected watermark to exist")
	}
	if wm != 2 {
		t.Errorf("expected watermark 2, got %d", wm)
	}
}

func TestMemoryRetryStore_ResetPartition(t *testing.T) {
	store := NewMemoryRetryStore()
	store.Schedule(context.Background(), &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, NextRetryAt: time.Now().Add(-time.Second),
	})
	store.ResetPartition("test", 0)
	_, ok := store.Watermark("test", 0)
	if ok {
		t.Error("expected watermark to not exist after reset")
	}
}

func TestMemoryRetryStore_Remove(t *testing.T) {
	store := NewMemoryRetryStore()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-1 * time.Second),
	}
	store.Schedule(context.Background(), item)
	store.Remove(context.Background(), item)
	items, err := store.Fetch(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items after remove, got %d", len(items))
	}
}

func TestMemoryRetryStore_Reschedule(t *testing.T) {
	store := NewMemoryRetryStore()
	now := time.Now()
	oldItem := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-1 * time.Second),
	}
	newItem := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 2, NextRetryAt: now.Add(5 * time.Second),
	}
	store.Schedule(context.Background(), oldItem)
	if err := store.Reschedule(context.Background(), oldItem, newItem); err != nil {
		t.Fatalf("Reschedule failed: %v", err)
	}
	// New item is not due yet
	items, _ := store.Fetch(context.Background(), now, 10)
	if len(items) != 0 {
		t.Errorf("expected 0 items for not-due rescheduled item, got %d", len(items))
	}
	// After the retry time, should get the new item with attempt 2
	items, _ = store.Fetch(context.Background(), now.Add(6*time.Second), 10)
	if len(items) != 1 {
		t.Fatalf("expected 1 item after reschedule, got %d", len(items))
	}
	if items[0].Attempt != 2 {
		t.Errorf("expected attempt 2, got %d", items[0].Attempt)
	}
}

func TestMemoryRetryStore_LoadAll(t *testing.T) {
	store := NewMemoryRetryStore()
	items, err := store.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil items for memory store, got %v", items)
	}
}

func TestMemoryRetryStore_Close(t *testing.T) {
	store := NewMemoryRetryStore()
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestMemoryRetryStore_QueueFull(t *testing.T) {
	store := NewMemoryRetryStore(WithMemoryMaxQueueSize(2))
	now := time.Now()

	item1 := &RetryItem{Topic: "test", Partition: 0, Offset: 1, NextRetryAt: now.Add(-time.Second)}
	item2 := &RetryItem{Topic: "test", Partition: 0, Offset: 2, NextRetryAt: now.Add(-time.Second)}
	item3 := &RetryItem{Topic: "test", Partition: 0, Offset: 3, NextRetryAt: now.Add(-time.Second)}

	if err := store.Schedule(context.Background(), item1); err != nil {
		t.Fatalf("Schedule item1 failed: %v", err)
	}
	if err := store.Schedule(context.Background(), item2); err != nil {
		t.Fatalf("Schedule item2 failed: %v", err)
	}
	if err := store.Schedule(context.Background(), item3); err == nil {
		t.Error("expected ErrRetryQueueFull for item3")
	} else if err != ErrRetryQueueFull {
		t.Errorf("expected ErrRetryQueueFull, got %v", err)
	}
}

func TestMemoryRetryStore_FetchLimit(t *testing.T) {
	store := NewMemoryRetryStore()
	now := time.Now()
	for i := 0; i < 5; i++ {
		store.Schedule(context.Background(), &RetryItem{
			Topic: "test", Partition: 0, Offset: int64(i), NextRetryAt: now.Add(-time.Second),
		})
	}
	items, err := store.Fetch(context.Background(), now, 2)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with limit=2, got %d", len(items))
	}
}

func TestMemoryRetryStore_Notify(t *testing.T) {
	store := NewMemoryRetryStore()
	ch := store.Notify()
	if ch == nil {
		t.Fatal("expected non-nil notify channel")
	}
}
