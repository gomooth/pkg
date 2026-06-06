package attempt_tracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageKey(t *testing.T) {
	key1 := MessageKey("test-data")
	key2 := MessageKey("test-data")
	key3 := MessageKey("other-data")

	assert.Equal(t, key1, key2, "same input should produce same key")
	assert.NotEqual(t, key1, key3, "different input should produce different key")
	assert.Len(t, key1, 16, "key should be 16 hex characters (8 bytes)")
}

func TestAttemptTracker_IncrementAndGet(t *testing.T) {
	tracker := NewAttemptTracker()
	defer tracker.Close()

	key := MessageKey("test-msg")

	attempt := tracker.Increment(key)
	assert.Equal(t, 1, attempt)

	attempt = tracker.Increment(key)
	assert.Equal(t, 2, attempt)

	got, ok := tracker.Get(key)
	assert.True(t, ok)
	assert.Equal(t, 2, got)
}

func TestAttemptTracker_Get_NotFound(t *testing.T) {
	tracker := NewAttemptTracker()
	defer tracker.Close()

	got, ok := tracker.Get("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, 0, got)
}

func TestAttemptTracker_Remove(t *testing.T) {
	tracker := NewAttemptTracker()
	defer tracker.Close()

	key := MessageKey("test-msg")
	tracker.Increment(key)
	tracker.Remove(key)

	got, ok := tracker.Get(key)
	assert.False(t, ok)
	assert.Equal(t, 0, got)
}

func TestAttemptTracker_CleanExpired(t *testing.T) {
	tracker := NewAttemptTracker(WithMaxAge(50 * time.Millisecond))
	defer tracker.Close()

	key := MessageKey("test-msg")
	tracker.Increment(key)

	// 条目存在
	_, ok := tracker.Get(key)
	assert.True(t, ok)

	// 等待过期
	time.Sleep(100 * time.Millisecond)
	tracker.CleanExpired(50 * time.Millisecond)

	// 条目已清理
	_, ok = tracker.Get(key)
	assert.False(t, ok)
}

func TestAttemptTracker_AutoCleanup(t *testing.T) {
	tracker := NewAttemptTracker(
		WithMaxAge(50*time.Millisecond),
		WithCleanInterval(30*time.Millisecond),
	)
	defer tracker.Close()

	key := MessageKey("test-msg")
	tracker.Increment(key)

	// 等待自动清理
	time.Sleep(150 * time.Millisecond)

	// 条目应被自动清理
	_, ok := tracker.Get(key)
	assert.False(t, ok)
}

func TestAttemptTracker_Close_Idempotent(t *testing.T) {
	tracker := NewAttemptTracker()
	assert.NotPanics(t, func() {
		tracker.Close()
		tracker.Close() // 重复调用不应 panic
	})
}

func TestAttemptTracker_WithOptions(t *testing.T) {
	tracker := NewAttemptTracker(
		WithMaxAge(10*time.Minute),
		WithCleanInterval(1*time.Minute),
	)
	defer tracker.Close()

	assert.NotNil(t, tracker)
}

func TestAttemptTracker_MultipleKeys(t *testing.T) {
	tracker := NewAttemptTracker()
	defer tracker.Close()

	key1 := MessageKey("msg-1")
	key2 := MessageKey("msg-2")

	tracker.Increment(key1)
	tracker.Increment(key1)
	tracker.Increment(key2)

	got1, ok1 := tracker.Get(key1)
	assert.True(t, ok1)
	assert.Equal(t, 2, got1)

	got2, ok2 := tracker.Get(key2)
	assert.True(t, ok2)
	assert.Equal(t, 1, got2)
}
