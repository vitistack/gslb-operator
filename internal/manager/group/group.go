package group

import (
	"sync"

	"github.com/vitistack/gslb-operator/pkg/iter"
)

type GroupUnlocker interface {
	Unlock()
}

type lockedGroup struct {
	lock  *sync.Mutex // per group access control
	group *ServiceGroupV2
}

type ServiceGroups struct {
	lock   sync.Mutex // global access to groups
	groups map[string]lockedGroup
}

func NewServiceGroups() *ServiceGroups {
	return &ServiceGroups{
		lock:   sync.Mutex{},
		groups: map[string]lockedGroup{},
	}
}

//func (g *ServiceGroups) Get(key string) ServiceGroup {
//	g.lock.Lock()
//	lGroup, ok := g.groups[key]
//	g.lock.Unlock()
//	if !ok {
//		return nil
//	}
//
//	lGroup.lock.Lock()
//	return lGroup.group
//}

// returns success true/false if the release was good or not
//func (g *ServiceGroups) Release(key string) {
//	g.lock.Lock()
//	lGroup, ok := g.groups[key]
//	g.lock.Unlock()
//	if !ok {
//		return
//	}
//	lGroup.lock.Unlock()
//}

// With locks the group for key, and invokes fn which is responsible for unlocking the group once it is finished
// returns false if there are no group for key
func (g *ServiceGroups) With(key string, fn func(ServiceGroup, GroupUnlocker)) bool {
	g.lock.Lock()
	lGroup, ok := g.groups[key]
	g.lock.Unlock()

	if !ok {
		return false
	}

	lGroup.lock.Lock()

	fn(lGroup.group, lGroup.lock)
	return true
}

func (g *ServiceGroups) Delete(key string) {
	g.lock.Lock()
	defer g.lock.Unlock()

	lGroup, ok := g.groups[key]
	if !ok {
		return
	}

	// need to fetch the lock to delete group
	// to be safe that no other thread is currently operating on this group
	lGroup.lock.Lock()
	delete(g.groups, key)
	lGroup.lock.Unlock()
}

func (g *ServiceGroups) Create(key string, init func(ServiceGroup)) bool {
	g.lock.Lock()
	defer g.lock.Unlock()

	if _, ok := g.groups[key]; ok {
		return false
	}

	newGroup := NewServiceGroup(key)
	if init != nil {
		init(newGroup)
	}

	g.groups[key] = lockedGroup{
		lock:  &sync.Mutex{},
		group: newGroup,
	}

	return true
}

// after each iteration a group is both locked and unlocked
// this means that it is unsafe to use the group reference after an iteration
func (g *ServiceGroups) Groups() iter.Iterator2[string, ServiceGroup] {
	return func(yield func(string, ServiceGroup) bool) {
		g.lock.Lock()
		defer g.lock.Unlock()

		for key, group := range g.groups {
			group.lock.Lock()
			if !yield(key, group.group) {
				group.lock.Unlock()
				return
			}
			group.lock.Unlock()
		}
	}
}
