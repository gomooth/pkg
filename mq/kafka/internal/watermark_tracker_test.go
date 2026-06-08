package internal

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/stretchr/testify/assert"
)

func TestWatermarkTracker_MarkSuccessAdvances(t *testing.T) {
	tracker := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))

	tracker.MarkSuccess("test", 0, 5)
	wm, ok := tracker.Watermark("test", 0)
	if !ok || wm != 5 {
		t.Errorf("expected watermark 5, got %d, ok=%v", wm, ok)
	}
}

func TestWatermarkTracker_PendingBlocksWatermark(t *testing.T) {
	tracker := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))

	// 标记 1,3 为 pending
	tracker.MarkPending("test", 0, 1)
	tracker.MarkPending("test", 0, 3)
	// 标记 2 成功
	tracker.MarkSuccess("test", 0, 2)

	// 水位线应为 0（pending 1 阻塞）
	wm, ok := tracker.Watermark("test", 0)
	if !ok || wm != 0 {
		t.Errorf("expected watermark 0 (blocked by pending 1), got %d", wm)
	}

	// 标记 1 成功 → pending 移除 1
	tracker.MarkSuccess("test", 0, 1)
	// 水位线推进：pending 3 阻塞 → wm = 2
	wm, ok = tracker.Watermark("test", 0)
	if !ok || wm != 2 {
		t.Errorf("expected watermark 2, got %d", wm)
	}
}

func TestWatermarkTracker_ResetPartition(t *testing.T) {
	tracker := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))

	tracker.MarkSuccess("test", 0, 5)
	wm, ok := tracker.Watermark("test", 0)
	if !ok {
		t.Fatal("expected watermark to exist")
	}
	if wm != 5 {
		t.Errorf("expected watermark 5, got %d", wm)
	}

	tracker.ResetPartition("test", 0)
	_, ok = tracker.Watermark("test", 0)
	if ok {
		t.Error("expected watermark to not exist after reset")
	}
}

func TestWatermarkTracker_MarkPendingOverflow(t *testing.T) {
	tracker := NewWatermarkTracker(nil) // nil logger → uses no-op logger

	// 超限应返回 false
	for i := int64(0); i < 10001; i++ {
		result := tracker.MarkPending("test", 0, i)
		if i < 10000 && !result {
			t.Fatalf("MarkPending(%d) should succeed", i)
		}
		if i >= 10000 && result {
			t.Fatalf("MarkPending(%d) should fail (overflow)", i)
		}
	}
}

func TestWatermarkTracker_RemovePending(t *testing.T) {
	tracker := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))

	t.Run("remove existing pending offset", func(t *testing.T) {
		tracker.MarkPending("remove-test", 0, 1)
		tracker.MarkPending("remove-test", 0, 3)
		tracker.MarkSuccess("remove-test", 0, 2)

		// Before removal: pending {1,3}, watermark should be 0 (blocked by 1)
		wm, ok := tracker.Watermark("remove-test", 0)
		assert.True(t, ok)
		assert.Equal(t, int64(0), wm)

		// Remove pending offset 1
		tracker.RemovePending("remove-test", 0, 1)

		// Now pending {3}, watermark should be 2
		wm, ok = tracker.Watermark("remove-test", 0)
		assert.True(t, ok)
		assert.Equal(t, int64(2), wm, "watermark should advance to 2 after removing pending 1")
	})

	t.Run("remove non-existing pending offset has no effect", func(t *testing.T) {
		tracker2 := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))
		tracker2.MarkSuccess("no-effect", 0, 5)
		wmBefore, okBefore := tracker2.Watermark("no-effect", 0)

		tracker2.RemovePending("no-effect", 0, 999) // non-existing offset

		wmAfter, okAfter := tracker2.Watermark("no-effect", 0)
		assert.Equal(t, okBefore, okAfter)
		assert.Equal(t, wmBefore, wmAfter, "removing non-existing pending should not change watermark")
	})

	t.Run("pending count reflects removal", func(t *testing.T) {
		tracker3 := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))
		tracker3.MarkPending("count-test", 0, 10)
		tracker3.MarkPending("count-test", 0, 20)
		assert.Equal(t, 2, tracker3.PendingCount("count-test", 0))

		tracker3.RemovePending("count-test", 0, 10)
		assert.Equal(t, 1, tracker3.PendingCount("count-test", 0), "PendingCount should be 1 after removing one pending")

		tracker3.RemovePending("count-test", 0, 20)
		assert.Equal(t, 0, tracker3.PendingCount("count-test", 0), "PendingCount should be 0 after removing all pending")
	})
}

func TestWatermarkTracker_PendingCount(t *testing.T) {
	tracker := NewWatermarkTracker(logutil.NewSlogLogger(slog.Default()))

	t.Run("no pending returns zero", func(t *testing.T) {
		assert.Equal(t, 0, tracker.PendingCount("empty", 0))
	})

	t.Run("correct count with pending items", func(t *testing.T) {
		tracker.MarkPending("count", 0, 1)
		tracker.MarkPending("count", 0, 2)
		tracker.MarkPending("count", 0, 3)
		assert.Equal(t, 3, tracker.PendingCount("count", 0))

		// MarkSuccess removes from pending
		tracker.MarkSuccess("count", 0, 2)
		assert.Equal(t, 2, tracker.PendingCount("count", 0))
	})
}

func TestWatermarkTracker_NoopSlogLogger(t *testing.T) {
	// Verify that NewWatermarkTracker(nil) does not panic and works correctly
	tracker := NewWatermarkTracker(nil)
	tracker.MarkSuccess("nil-logger", 0, 5)
	wm, ok := tracker.Watermark("nil-logger", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(5), wm)
}

func TestDiscardHandler(t *testing.T) {
	h := discardHandler{}

	t.Run("Enabled returns false", func(t *testing.T) {
		assert.False(t, h.Enabled(nil, 0))
	})

	t.Run("Handle returns nil", func(t *testing.T) {
		err := h.Handle(context.Background(), slog.Record{})
		assert.Nil(t, err)
	})

	t.Run("WithAttrs returns self", func(t *testing.T) {
		result := h.WithAttrs(nil)
		assert.Equal(t, h, result)
	})

	t.Run("WithGroup returns self", func(t *testing.T) {
		result := h.WithGroup("test")
		assert.Equal(t, h, result)
	})
}
