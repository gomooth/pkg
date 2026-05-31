package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisStore(t *testing.T) (*RedisRetryStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client)
	return store, mr
}

func TestRedisRetryStore_ScheduleAndFetch(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-1 * time.Second), ConsumerGroup: "test-group",
	}
	ctx := context.Background()
	if err := store.Schedule(ctx, item); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	items, err := store.Fetch(ctx, now, 10)
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

func TestRedisRetryStore_FetchNotDue(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(10 * time.Second), ConsumerGroup: "test-group",
	}
	ctx := context.Background()
	store.Schedule(ctx, item)
	items, err := store.Fetch(ctx, now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestRedisRetryStore_Remove(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	store.Schedule(ctx, item)
	store.Remove(ctx, item)
	items, _ := store.Fetch(ctx, now, 10)
	if len(items) != 0 {
		t.Errorf("expected 0 items after remove, got %d", len(items))
	}
}

func TestRedisRetryStore_Reschedule(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()
	oldItem := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	newItem := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 2, NextRetryAt: now.Add(5 * time.Second), ConsumerGroup: "test-group",
	}
	store.Schedule(ctx, oldItem)
	store.Reschedule(ctx, oldItem, newItem)
	items, _ := store.Fetch(ctx, now, 10)
	if len(items) != 0 {
		t.Errorf("expected 0 items for not-due rescheduled item, got %d", len(items))
	}
	items, _ = store.Fetch(ctx, now.Add(6*time.Second), 10)
	if len(items) != 1 {
		t.Fatalf("expected 1 item after reschedule, got %d", len(items))
	}
	if items[0].Attempt != 2 {
		t.Errorf("expected attempt 2, got %d", items[0].Attempt)
	}
}

func TestRedisRetryStore_LoadAll(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()

	item1 := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello1"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	item2 := &RetryItem{
		Topic: "test", Partition: 0, Offset: 2, Value: []byte("hello2"),
		Attempt: 1, NextRetryAt: now.Add(5 * time.Second), ConsumerGroup: "test-group",
	}
	store.Schedule(ctx, item1)
	store.Schedule(ctx, item2)

	items, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items from LoadAll, got %d", len(items))
	}
}

func TestRedisRetryStore_WithKeyPrefix(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client, WithRedisKeyPrefix("custom:prefix"))
	defer store.Close()

	ctx := context.Background()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	if err := store.Schedule(ctx, item); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	items, err := store.Fetch(ctx, now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestRedisRetryStore_WithFetchLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client, WithRedisFetchLimit(1))
	defer store.Close()

	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 3; i++ {
		store.Schedule(ctx, &RetryItem{
			Topic: "test", Partition: 0, Offset: int64(i), Value: []byte("hello"),
			Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
		})
	}
	items, err := store.Fetch(ctx, now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item with fetchLimit=1, got %d", len(items))
	}
}

func TestRedisRetryStore_Close(t *testing.T) {
	store, _ := newTestRedisStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestRedisRetryStore_ScheduleWithHeaders(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Key:     []byte("my-key"),
		Headers: []HeaderKV{{Key: "trace-id", Value: []byte("12345")}},
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	store.Schedule(ctx, item)
	items, err := store.Fetch(ctx, now, 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(items[0].Headers))
	}
	if items[0].Headers[0].Key != "trace-id" {
		t.Errorf("expected header key 'trace-id', got %q", items[0].Headers[0].Key)
	}
	if string(items[0].Headers[0].Value) != "12345" {
		t.Errorf("expected header value '12345', got %q", string(items[0].Headers[0].Value))
	}
	if string(items[0].Key) != "my-key" {
		t.Errorf("expected key 'my-key', got %q", string(items[0].Key))
	}
}
