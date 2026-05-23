package internal

import "container/heap"

// retryHeap 基于 NextRetryAt 的最小堆，用于 Plan B 的内存优先队列
type retryHeap struct {
	data         []*RetryItem
	maxQueueSize int // 最大队列容量，0 表示无限制
}

func newRetryHeap(maxQueueSize int) *retryHeap {
	h := &retryHeap{maxQueueSize: maxQueueSize}
	heap.Init(h)
	return h
}

func (h retryHeap) Len() int           { return len(h.data) }
func (h retryHeap) Less(i, j int) bool { return h.data[i].NextRetryAt.Before(h.data[j].NextRetryAt) }
func (h retryHeap) Swap(i, j int)      { h.data[i], h.data[j] = h.data[j], h.data[i] }

func (h *retryHeap) Push(x any) {
	h.data = append(h.data, x.(*RetryItem))
}

func (h *retryHeap) Pop() any {
	old := h.data
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	h.data = old[:n-1]
	return item
}

// IsFull 检查队列是否已达容量上限
func (h *retryHeap) IsFull() bool {
	return h.maxQueueSize > 0 && len(h.data) >= h.maxQueueSize
}

// PushItem 向堆中添加一个重试项
func (h *retryHeap) PushItem(item *RetryItem) {
	heap.Push(h, item)
}

// PopItem 弹出最早到期的重试项
func (h *retryHeap) PopItem() *RetryItem {
	if h.Len() == 0 {
		return nil
	}
	return heap.Pop(h).(*RetryItem)
}

// Peek 查看最早到期的重试项，不弹出
func (h *retryHeap) Peek() *RetryItem {
	if h.Len() == 0 {
		return nil
	}
	return h.data[0]
}
