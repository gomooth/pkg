package internal

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// AttemptTracker 再入队模式的消息重试次数跟踪器
type AttemptTracker struct {
	mu      sync.Mutex
	entries map[string]*attemptEntry
}

type attemptEntry struct {
	attempt    int
	lastSeenAt time.Time
}

// NewAttemptTracker 创建重试次数跟踪器
func NewAttemptTracker() *AttemptTracker {
	return &AttemptTracker{
		entries: make(map[string]*attemptEntry),
	}
}

// MessageKey 根据消息内容生成跟踪 key（sha256 前 16 字符）
func MessageKey(data string) string {
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:8])
}

// Increment 递增消息的重试次数，返回当前次数（递增后）
func (t *AttemptTracker) Increment(key string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[key]
	if !ok {
		e = &attemptEntry{attempt: 0, lastSeenAt: time.Now()}
		t.entries[key] = e
	}
	e.attempt++
	e.lastSeenAt = time.Now()
	return e.attempt
}

// Get 获取消息的当前重试次数
func (t *AttemptTracker) Get(key string) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[key]
	if !ok {
		return 0, false
	}
	return e.attempt, true
}

// Remove 移除消息的跟踪记录
func (t *AttemptTracker) Remove(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entries, key)
}

// CleanExpired 清理超过 maxAge 未访问的条目
func (t *AttemptTracker) CleanExpired(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for k, e := range t.entries {
		if now.Sub(e.lastSeenAt) > maxAge {
			delete(t.entries, k)
		}
	}
}
