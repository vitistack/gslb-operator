package scheduler

import (
	"slices"
)

type ItemHeap[T any] []*ScheduledItem[T]

func (h ItemHeap[T]) Len() int {
	return len(h)
}

func (h ItemHeap[T]) Less(i, j int) bool {
	return h[i].nextCheckTime.Before(h[j].nextCheckTime)
}

func (h ItemHeap[T]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *ItemHeap[T]) Push(x any) {
	*h = append(*h, x.(*ScheduledItem[T]))
}

func (h *ItemHeap[T]) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return item
}

func (h ItemHeap[T]) Peek() *ScheduledItem[T] {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}

func (h *ItemHeap[T]) GetIndex(cmp func(*ScheduledItem[T]) bool) int {
	return slices.IndexFunc(*h, cmp)
}
