package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWatermarkTracker_BasicFlow(t *testing.T) {
	wt := NewWatermarkTracker()

	for i := int64(0); i < 5; i++ {
		wt.MarkSuccess("topic", 0, i)
	}

	wm, ok := wt.Watermark("topic", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(4), wm)
}

func TestWatermarkTracker_PendingBlocksWatermark(t *testing.T) {
	wt := NewWatermarkTracker()

	wt.MarkSuccess("topic", 0, 0)
	wt.MarkSuccess("topic", 0, 1)
	wt.MarkPending("topic", 0, 2)
	wt.MarkSuccess("topic", 0, 3)

	wm, ok := wt.Watermark("topic", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(1), wm, "watermark should stop at offset 1 (2 is pending)")
}

func TestWatermarkTracker_ResolvePendingAdvancesWatermark(t *testing.T) {
	wt := NewWatermarkTracker()

	wt.MarkSuccess("topic", 0, 0)
	wt.MarkPending("topic", 0, 1)
	wt.MarkSuccess("topic", 0, 2)

	wm, _ := wt.Watermark("topic", 0)
	assert.Equal(t, int64(0), wm)

	wt.MarkSuccess("topic", 0, 1)
	wm, _ = wt.Watermark("topic", 0)
	assert.Equal(t, int64(2), wm)
}

func TestWatermarkTracker_LargeGapPerformance(t *testing.T) {
	wt := NewWatermarkTracker()

	wt.MarkSuccess("topic", 0, 0)
	wt.MarkPending("topic", 0, 1)

	start := time.Now()
	wt.MarkSuccess("topic", 0, 1000000)
	elapsed := time.Since(start)

	wm, ok := wt.Watermark("topic", 0)
	assert.True(t, ok)
	assert.Equal(t, int64(0), wm, "watermark should be 0 (offset 1 is pending)")

	// 性能断言：应在 10ms 内完成，O(n) 扫描需要 >> 10ms
	assert.Less(t, elapsed, 10*time.Millisecond, "large gap recomputation should be fast")

	wt.MarkSuccess("topic", 0, 1)
	wm, _ = wt.Watermark("topic", 0)
	assert.Equal(t, int64(1000000), wm)
}

func TestWatermarkTracker_RemovePendingAdvancesWatermark(t *testing.T) {
	wt := NewWatermarkTracker()

	wt.MarkSuccess("topic", 0, 0)
	wt.MarkPending("topic", 0, 1)
	wt.MarkSuccess("topic", 0, 2)

	wm, _ := wt.Watermark("topic", 0)
	assert.Equal(t, int64(0), wm)

	wt.RemovePending("topic", 0, 1)
	wm, _ = wt.Watermark("topic", 0)
	assert.Equal(t, int64(2), wm)
}

func TestWatermarkTracker_ResetPartition(t *testing.T) {
	wt := NewWatermarkTracker()

	wt.MarkSuccess("topic", 0, 10)
	_, ok := wt.Watermark("topic", 0)
	assert.True(t, ok)

	wt.ResetPartition("topic", 0)
	_, ok = wt.Watermark("topic", 0)
	assert.False(t, ok, "watermark should not exist after reset")
}

func TestWatermarkTracker_MultiplePartitions(t *testing.T) {
	wt := NewWatermarkTracker()

	wt.MarkSuccess("topic", 0, 5)
	wt.MarkSuccess("topic", 1, 10)

	wm0, _ := wt.Watermark("topic", 0)
	wm1, _ := wt.Watermark("topic", 1)
	assert.Equal(t, int64(5), wm0)
	assert.Equal(t, int64(10), wm1)
}
