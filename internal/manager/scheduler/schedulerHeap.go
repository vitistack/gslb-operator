package scheduler

import (
	"github.com/vitistack/gslb-operator/internal/service"
)

type ServiceHeap []*ScheduledService

func (h ServiceHeap) Len() int {
	return len(h)
}

func (h ServiceHeap) Less(i, j int) bool {
	return h[i].nextCheckTime.Before(h[j].nextCheckTime)
}

func (h ServiceHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *ServiceHeap) Push(x any) {
	*h = append(*h, x.(*ScheduledService))
}

func (h *ServiceHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return item
}

func (h ServiceHeap) Peek() *ScheduledService {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}

func (h *ServiceHeap) GetServiceIndex(service *service.Service) int {
	for index, scheduled := range *h {
		if scheduled.service.Fqdn == service.Fqdn {
			return index
		}
	}
	return -1
}
