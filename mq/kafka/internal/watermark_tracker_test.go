package internal

import (
	"log/slog"
	"testing"
)

func TestWatermarkTracker_MarkSuccessAdvances(t *testing.T) {
	tracker := NewWatermarkTracker(NewSlogLogger(slog.Default()))

	tracker.MarkSuccess("test", 0, 5)
	wm, ok := tracker.Watermark("test", 0)
	if !ok || wm != 5 {
		t.Errorf("expected watermark 5, got %d, ok=%v", wm, ok)
	}
}

func TestWatermarkTracker_PendingBlocksWatermark(t *testing.T) {
	tracker := NewWatermarkTracker(NewSlogLogger(slog.Default()))

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
	tracker := NewWatermarkTracker(NewSlogLogger(slog.Default()))

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
