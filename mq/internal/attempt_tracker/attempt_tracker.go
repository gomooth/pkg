package attempt_tracker

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AttemptTracker 再入队模式的消息重试次数跟踪器。
// 内置自动清理机制，定期清除过期条目，防止长时间运行导致的内存泄漏。
type AttemptTracker struct {
	mu      sync.Mutex
	entries map[string]*attemptEntry
	closed  atomic.Bool
	done    chan struct{}

	maxAge        time.Duration
	cleanInterval time.Duration
}

type attemptEntry struct {
	attempt    int
	lastSeenAt time.Time
}

// AttemptTrackerOption AttemptTracker 的配置选项
type AttemptTrackerOption struct {
	maxAge        time.Duration
	cleanInterval time.Duration
}

// WithMaxAge 设置过期条目的最大存活时间（默认 30 分钟）
func WithMaxAge(d time.Duration) func(*AttemptTrackerOption) {
	return func(o *AttemptTrackerOption) { o.maxAge = d }
}

// WithCleanInterval 设置自动清理间隔（默认 5 分钟）
func WithCleanInterval(d time.Duration) func(*AttemptTrackerOption) {
	return func(o *AttemptTrackerOption) { o.cleanInterval = d }
}

// NewAttemptTracker 创建重试次数跟踪器。
// 自动启动后台清理 goroutine，调用方应在消费者 Shutdown 时调用 Close() 停止。
func NewAttemptTracker(opts ...func(*AttemptTrackerOption)) *AttemptTracker {
	o := &AttemptTrackerOption{
		maxAge:        30 * time.Minute,
		cleanInterval: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}

	t := &AttemptTracker{
		entries:       make(map[string]*attemptEntry),
		done:          make(chan struct{}),
		maxAge:        o.maxAge,
		cleanInterval: o.cleanInterval,
	}
	go t.cleanupLoop()
	return t
}

// Close 停止自动清理 goroutine。
// 应在消费者 Shutdown 时调用。
func (t *AttemptTracker) Close() {
	if t.closed.CompareAndSwap(false, true) {
		close(t.done)
	}
}

func (t *AttemptTracker) cleanupLoop() {
	ticker := time.NewTicker(t.cleanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.CleanExpired(t.maxAge)
		case <-t.done:
			return
		}
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
