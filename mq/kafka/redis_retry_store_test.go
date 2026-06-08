package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisStore(t *testing.T) (*RedisRetryStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client)
	return store, mr
}

// newTestRedisStoreWithClient creates a store with direct access to the redis client for setup
func newTestRedisStoreWithClient(t *testing.T) (*RedisRetryStore, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client)
	return store, mr, client
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

// ==================== 新增边界测试 ====================

func TestRedisRetryStore_FetchOrphanedKeyCleanup(t *testing.T) {
	// Scenario: schedule key has an entry, but the corresponding data key has no hash fields
	// (orphaned key). Fetch should clean it up and return no items.
	store, _, client := newTestRedisStoreWithClient(t)
	defer store.Close()
	ctx := context.Background()

	// Manually add an entry to the schedule sorted set without a corresponding data hash
	orphanDKey := store.dataKey("orphan-topic", 0, 99)
	err := client.ZAdd(ctx, store.scheduleKey(), redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: orphanDKey,
	}).Err()
	require.NoError(t, err)

	items, err := store.Fetch(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for orphaned key, got %d", len(items))
	}

	// Verify the orphaned schedule entry was removed
	vals, err := client.ZRange(ctx, store.scheduleKey(), 0, -1).Result()
	require.NoError(t, err)
	if len(vals) != 0 {
		t.Errorf("expected orphaned key to be cleaned from schedule, got %v", vals)
	}
}

func TestRedisRetryStore_FetchInvalidDataCleanup(t *testing.T) {
	// Scenario: schedule key and data key both exist, but the data hash contains
	// invalid fields that cannot be parsed (e.g., non-numeric partition).
	// Fetch should skip the item, clean it up, and return no items.
	store, mr, client := newTestRedisStoreWithClient(t)
	defer store.Close()
	ctx := context.Background()

	dKey := store.dataKey("bad-topic", 0, 1)

	// Add schedule entry
	err := client.ZAdd(ctx, store.scheduleKey(), redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: dKey,
	}).Err()
	require.NoError(t, err)

	// Add data hash with invalid partition value
	err = client.HSet(ctx, dKey, map[string]interface{}{
		"topic": "bad-topic", "partition": "not-a-number", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
	}).Err()
	require.NoError(t, err)

	items, err := store.Fetch(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for invalid data, got %d", len(items))
	}

	// Verify the orphaned data key was cleaned
	exists := mr.Exists(dKey)
	if exists {
		t.Error("expected orphaned data key to be deleted")
	}
}

func TestRedisRetryStore_FetchDefaultLimit(t *testing.T) {
	// When limit is 0 or negative, it should use the store's default fetchLimit
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()

	// Schedule one item
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	store.Schedule(ctx, item)

	// Fetch with limit=0 should use default
	items, err := store.Fetch(ctx, now, 0)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item with limit=0, got %d", len(items))
	}
}

func TestRedisRetryStore_FetchLimitExceedsStoreLimit(t *testing.T) {
	// When limit exceeds store's fetchLimit, it should be capped
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

	// Request more than fetchLimit
	items, err := store.Fetch(ctx, now, 100)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (capped by fetchLimit=1), got %d", len(items))
	}
}

func TestRedisRetryStore_FetchNegativeLimit(t *testing.T) {
	// When limit is negative, it should use the store's default fetchLimit
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()

	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "test-group",
	}
	store.Schedule(ctx, item)

	items, err := store.Fetch(ctx, now, -5)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item with negative limit, got %d", len(items))
	}
}

func TestRedisRetryStore_LoadAllEmpty(t *testing.T) {
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()

	items, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty store, got %d", len(items))
	}
}

func TestRedisRetryStore_LoadAllOrphanedKeyCleanup(t *testing.T) {
	// Scenario: schedule key has an entry but data hash is empty or invalid.
	// LoadAll should clean up orphaned entries.
	store, _, client := newTestRedisStoreWithClient(t)
	defer store.Close()
	ctx := context.Background()

	// Add valid item
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: time.Now().Add(-time.Second), ConsumerGroup: "g",
	}
	store.Schedule(ctx, item)

	// Add orphaned schedule entry (no corresponding data hash)
	orphanKey := store.dataKey("orphan", 0, 99)
	err := client.ZAdd(ctx, store.scheduleKey(), redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: orphanKey,
	}).Err()
	require.NoError(t, err)

	items, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 valid item, got %d", len(items))
	}

	// Verify orphaned key was cleaned from schedule
	vals, err := client.ZRange(ctx, store.scheduleKey(), 0, -1).Result()
	require.NoError(t, err)
	for _, v := range vals {
		if v == orphanKey {
			t.Error("expected orphaned key to be removed from schedule")
		}
	}
}

func TestRedisRetryStore_LoadAllInvalidDataCleanup(t *testing.T) {
	// Scenario: data hash exists but has invalid fields.
	// LoadAll should treat it as orphaned and clean it up.
	store, _, client := newTestRedisStoreWithClient(t)
	defer store.Close()
	ctx := context.Background()

	badKey := store.dataKey("bad", 0, 1)
	err := client.ZAdd(ctx, store.scheduleKey(), redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: badKey,
	}).Err()
	require.NoError(t, err)
	// Set invalid data in hash (partition is not a number)
	err = client.HSet(ctx, badKey, map[string]interface{}{
		"topic": "bad", "partition": "invalid", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
	}).Err()
	require.NoError(t, err)

	items, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for invalid data, got %d", len(items))
	}
}

func TestRedisRetryStore_fromRedisFields_InvalidPartition(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "invalid", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid partition")
}

func TestRedisRetryStore_fromRedisFields_InvalidOffset(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "invalid",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid offset")
}

func TestRedisRetryStore_fromRedisFields_InvalidAttempt(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "invalid", "nextRetryAt": "1000", "consumerGroup": "g",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid attempt")
}

func TestRedisRetryStore_fromRedisFields_InvalidNextRetryAt(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "invalid", "consumerGroup": "g",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid nextRetryAt")
}

func TestRedisRetryStore_fromRedisFields_InvalidKeyBase64(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
		"key": "!!!invalid-base64!!!",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key base64")
}

func TestRedisRetryStore_fromRedisFields_InvalidValueBase64(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
		"value": "!!!invalid-base64!!!",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value base64")
}

func TestRedisRetryStore_fromRedisFields_InvalidHeadersJSON(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
		"headers": "not-json",
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid headers json")
}

func TestRedisRetryStore_fromRedisFields_InvalidHeaderKeyBase64(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
		"headers": `[{"key":"!!!invalid!!!","value":"dGVzdA=="}]`,
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid header key base64")
}

func TestRedisRetryStore_fromRedisFields_InvalidHeaderValueBase64(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
		"headers": `[{"key":"dGVzdA==","value":"!!!invalid!!!"}]`,
	}
	_, err := store.fromRedisFields(fields)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid header value base64")
}

func TestRedisRetryStore_fromRedisFields_EmptyHeaders(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
		"headers": "",
	}
	item, err := store.fromRedisFields(fields)
	assert.NoError(t, err)
	assert.Nil(t, item.Headers)
}

func TestRedisRetryStore_fromRedisFields_NoKeyNoValue(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]string{
		"topic": "test", "partition": "0", "offset": "1",
		"attempt": "1", "nextRetryAt": "1000", "consumerGroup": "g",
	}
	item, err := store.fromRedisFields(fields)
	assert.NoError(t, err)
	assert.Nil(t, item.Key)
	assert.Nil(t, item.Value)
}

func TestRedisRetryStore_flattenFields(t *testing.T) {
	store, _ := newTestRedisStore(t)
	fields := map[string]interface{}{
		"topic": "test",
		"count": 42,
	}
	result := store.flattenFields(fields)
	assert.Len(t, result, 4) // 2 keys * 2 (key + value)
}

func TestRedisRetryStore_RescheduleDifferentKeys(t *testing.T) {
	// Reschedule with different topic/partition/offset (different data keys)
	store, _ := newTestRedisStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Now()

	oldItem := &RetryItem{
		Topic: "test1", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "g",
	}
	newItem := &RetryItem{
		Topic: "test2", Partition: 1, Offset: 2, Value: []byte("world"),
		Attempt: 2, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "g",
	}

	store.Schedule(ctx, oldItem)
	err := store.Reschedule(ctx, oldItem, newItem)
	assert.NoError(t, err)

	// The new item should be available
	items, err := store.Fetch(ctx, now, 10)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "test2", items[0].Topic)
	assert.Equal(t, int32(1), items[0].Partition)
	assert.Equal(t, int64(2), items[0].Offset)
	assert.Equal(t, 2, items[0].Attempt)
}

func TestRedisRetryStore_ScheduleWithRetryStoreError(t *testing.T) {
	// Test Schedule when redis is unreachable
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client)

	ctx := context.Background()
	now := time.Now()
	item := &RetryItem{
		Topic: "test", Partition: 0, Offset: 1, Value: []byte("hello"),
		Attempt: 1, NextRetryAt: now.Add(-time.Second), ConsumerGroup: "g",
	}

	// Close miniredis to simulate connection failure
	mr.Close()

	err := store.Schedule(ctx, item)
	assert.Error(t, err)
}

func TestRedisRetryStore_FetchWithRetryStoreError(t *testing.T) {
	// Test Fetch when redis becomes unreachable
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client)

	// Close miniredis to simulate connection failure
	mr.Close()

	_, err := store.Fetch(context.Background(), time.Now(), 10)
	assert.Error(t, err)
}

func TestRedisRetryStore_RemoveWithRetryStoreError(t *testing.T) {
	// Test Remove when redis becomes unreachable
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisRetryStore(client)

	// Close miniredis to simulate connection failure
	mr.Close()

	item := &RetryItem{Topic: "test", Partition: 0, Offset: 1}
	err := store.Remove(context.Background(), item)
	assert.Error(t, err)
}
