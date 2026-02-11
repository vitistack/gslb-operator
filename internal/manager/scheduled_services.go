package manager

import (
	"cmp"
	"slices"

	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

type ScheduledServices map[timesutil.Duration][]*service.Service

func (s *ScheduledServices) Add(svc *service.Service) {
	queue, ok := (*s)[svc.ScheduledInterval]
	if !ok {
		queue = make([]*service.Service, 0)
	}

	queue = append(queue, svc)
	slices.SortFunc(queue, func(a, b *service.Service) int { // sorting the slice based on the ID of the service
		return cmp.Compare(a.GetID(), b.GetID())
	})

	(*s)[svc.ScheduledInterval] = queue
}

// removes a service completely from the datastructure
func (s *ScheduledServices) Delete(id string) {
	idx, interval, svc := s.Search(id)
	if idx == -1 {
		return // not found
	}

	(*s)[interval] = slices.DeleteFunc((*s)[interval], func(s *service.Service) bool {
		return s.GetID() == svc.GetID()
	})
}

// moves a service from one interval to another
// and returns wether the old interval is empty
func (s *ScheduledServices) MoveToInterval(svc *service.Service, newInterval timesutil.Duration) {
	svcID := svc.GetID()
	idx, _, svc := s.Search(svc.GetID())

	if idx == -1 { // not found
		return
	}

	// no call to Delete() because we have already searched through the datastructure once!
	(*s)[svc.ScheduledInterval] = slices.DeleteFunc((*s)[svc.ScheduledInterval], func(s *service.Service) bool { // remove it from its existing queue
		return s.GetID() == svcID
	})
	oldInterval := svc.ScheduledInterval
	svc.ScheduledInterval = newInterval
	s.Add(svc)

	if len((*s)[oldInterval]) == 0 {
		delete(*s, oldInterval)
	}
}

func (s *ScheduledServices) IntervalExists(interval timesutil.Duration) bool {
	_, ok := (*s)[interval]
	return ok
}

// searches each interval in the datastructure for the given ID
func (s *ScheduledServices) Search(id string) (int, timesutil.Duration, *service.Service) {
	for interval := range *s {
		idx, svc := s.SearchInterval(id, interval)
		if svc != nil {
			return idx, interval, svc
		}
	}

	return -1, 0, nil
}

// uses binary search to find the given id on an interval
func (s *ScheduledServices) SearchInterval(id string, interval timesutil.Duration) (int, *service.Service) {
	queue, ok := (*s)[interval]
	if !ok {
		return -1, nil
	}

	idx, ok := slices.BinarySearchFunc(queue, id, func(s *service.Service, target string) int {
		return cmp.Compare(s.GetID(), target)
	})

	if ok {
		return idx, queue[idx]
	}

	return -1, nil
}
