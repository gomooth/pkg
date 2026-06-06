package internal

import "container/heap"

// itemKey 用于在 index map 中唯一标识一个重试项
type itemKey struct {
	Topic     string
	Partition int32
	Offset    int64
}

// RetryHeap 基于 NextRetryAt 的最小堆，用于内存优先队列
type RetryHeap struct {
	data         []*RetryItem
	index        map[itemKey]int // itemKey -> 堆中索引，加速 Remove
	maxQueueSize int             // 最大队列容量，0 表示无限制
}

// NewRetryHeap 创建优先队列
func NewRetryHeap(maxQueueSize int) *RetryHeap {
	h := &RetryHeap{
		index:        make(map[itemKey]int),
		maxQueueSize: maxQueueSize,
	}
	heap.Init(h)
	return h
}

func (h RetryHeap) Len() int           { return len(h.data) }
func (h RetryHeap) Less(i, j int) bool { return h.data[i].NextRetryAt.Before(h.data[j].NextRetryAt) }

func (h RetryHeap) Swap(i, j int) {
	h.data[i], h.data[j] = h.data[j], h.data[i]
	h.index[itemKey{h.data[i].Topic, h.data[i].Partition, h.data[i].Offset}] = i
	h.index[itemKey{h.data[j].Topic, h.data[j].Partition, h.data[j].Offset}] = j
}

func (h *RetryHeap) Push(x any) {
	item := x.(*RetryItem)
	h.data = append(h.data, item)
	h.index[itemKey{item.Topic, item.Partition, item.Offset}] = len(h.data) - 1
}

func (h *RetryHeap) Pop() any {
	old := h.data
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	h.data = old[:n-1]
	delete(h.index, itemKey{item.Topic, item.Partition, item.Offset})
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
	key := itemKey{topic, partition, offset}
	idx, ok := h.index[key]
	if !ok {
		return false
	}
	heap.Remove(h, idx)
	return true
}
