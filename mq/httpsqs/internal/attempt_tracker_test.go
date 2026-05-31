package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageKey(t *testing.T) {
	key1 := MessageKey("hello")
	key2 := MessageKey("hello")
	key3 := MessageKey("world")

	assert.Equal(t, key1, key2, "same content should produce same key")
	assert.NotEqual(t, key1, key3, "different content should produce different key")
	assert.Len(t, key1, 16, "key should be 16 hex chars (8 bytes)")
}

func TestAttemptTracker_Increment(t *testing.T) {
	tracker := NewAttemptTracker()

	n := tracker.Increment("key1")
	assert.Equal(t, 1, n)

	n = tracker.Increment("key1")
	assert.Equal(t, 2, n)

	n = tracker.Increment("key2")
	assert.Equal(t, 1, n)
}

func TestAttemptTracker_Get(t *testing.T) {
	tracker := NewAttemptTracker()

	_, ok := tracker.Get("key1")
	assert.False(t, ok)

	tracker.Increment("key1")
	n, ok := tracker.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 1, n)
}

func TestAttemptTracker_Remove(t *testing.T) {
	tracker := NewAttemptTracker()

	tracker.Increment("key1")
	tracker.Remove("key1")

	_, ok := tracker.Get("key1")
	assert.False(t, ok)
}

func TestAttemptTracker_CleanExpired(t *testing.T) {
	tracker := NewAttemptTracker()

	// 手动设置一个过期的条目
	tracker.mu.Lock()
	tracker.entries["old"] = &attemptEntry{
		attempt:    3,
		lastSeenAt: time.Now().Add(-2 * time.Hour),
	}
	tracker.entries["fresh"] = &attemptEntry{
		attempt:    1,
		lastSeenAt: time.Now(),
	}
	tracker.mu.Unlock()

	tracker.CleanExpired(1 * time.Hour)

	_, ok := tracker.Get("old")
	assert.False(t, ok, "expired entry should be cleaned")

	_, ok = tracker.Get("fresh")
	assert.True(t, ok, "fresh entry should remain")
}
