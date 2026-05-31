package internal

import "container/heap"

// RetryHeap 基于 NextRetryAt 的最小堆，用于内存优先队列
type RetryHeap struct {
	data         []*RetryItem
	maxQueueSize int // 最大队列容量，0 表示无限制
}

// NewRetryHeap 创建优先队列
func NewRetryHeap(maxQueueSize int) *RetryHeap {
	h := &RetryHeap{maxQueueSize: maxQueueSize}
	heap.Init(h)
	return h
}

func (h RetryHeap) Len() int           { return len(h.data) }
func (h RetryHeap) Less(i, j int) bool { return h.data[i].NextRetryAt.Before(h.data[j].NextRetryAt) }
func (h RetryHeap) Swap(i, j int)      { h.data[i], h.data[j] = h.data[j], h.data[i] }

func (h *RetryHeap) Push(x any) {
	h.data = append(h.data, x.(*RetryItem))
}

func (h *RetryHeap) Pop() any {
	old := h.data
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	h.data = old[:n-1]
	return item
}

// IsFull 检查队列是否已达容量上限
func (h *RetryHeap) IsFull() bool {
	return h.maxQueueSize > 0 && len(h.data) >= h.maxQueueSize
}

// PushItem 向堆中添加一个重试项
func (h *RetryHeap) PushItem(item *RetryItem) {
	heap.Push(h, item)
}

// PopItem 弹出最早到期的重试项
func (h *RetryHeap) PopItem() *RetryItem {
	if h.Len() == 0 {
		return nil
	}
	return heap.Pop(h).(*RetryItem)
}

// Peek 查看最早到期的重试项，不弹出
func (h *RetryHeap) Peek() *RetryItem {
	if h.Len() == 0 {
		return nil
	}
	return h.data[0]
}

// Remove 从堆中移除指定项（按 Topic/Partition/Offset 匹配）。
// 如果找不到匹配项则返回 false。
func (h *RetryHeap) Remove(topic string, partition int32, offset int64) bool {
	for i, item := range h.data {
		if item.Topic == topic && item.Partition == partition && item.Offset == offset {
			heap.Remove(h, i)
			return true
		}
	}
	return false
}
