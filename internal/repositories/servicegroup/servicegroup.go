package servicegroup

import (
	"fmt"
	"sync"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type ServiceGroupRepo struct {
	lock  *sync.Mutex
	store persistence.Store[model.GSLBServiceGroup]
}

func NewServiceGroupRepo(store persistence.Store[model.GSLBServiceGroup]) *ServiceGroupRepo {
	return &ServiceGroupRepo{
		lock:  &sync.Mutex{},
		store: store,
	}
}

func (sr *ServiceGroupRepo) Create(memberOf string, group *model.GSLBServiceGroup) error {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	if err := sr.store.Save(memberOf, *group); err != nil {
		return fmt.Errorf("failed to create service group: %w", err)
	}
	return nil
}

func (sr *ServiceGroupRepo) Read(memberOf string) (model.GSLBServiceGroup, error) {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	if group, err := sr.store.Load(memberOf); err != nil {
		return model.GSLBServiceGroup{}, fmt.Errorf("failed to read from storage: %w", err)
	} else {
		return group, nil
	}
}

func (sr *ServiceGroupRepo) ReadAll() (iter.Iterator[model.GSLBServiceGroup], func() error) {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	it, finish := sr.store.LoadAll()
	return iter.FromSeq(it), finish
}

func (sr *ServiceGroupRepo) Mutate(memberOf string, mut func(*model.GSLBServiceGroup)) error {
	sr.lock.Lock()
	defer sr.lock.Unlock()

	group, err := sr.store.Load(memberOf)
	if err != nil {
		return err
	}

	mut(&group)

	if err := sr.store.Save(memberOf, group); err != nil {
		return fmt.Errorf("could not update group: %w", err)
	}

	return nil
}

func (sr *ServiceGroupRepo) Update(memberOf string, group *model.GSLBServiceGroup) error {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	if err := sr.store.Save(memberOf, *group); err != nil {
		return fmt.Errorf("could not update group: %s: %w", memberOf, err)
	}
	return nil
}

func (sr *ServiceGroupRepo) UpdateMember(memberOf string, svc model.GSLBService) error {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	group, err := sr.store.Load(memberOf)
	if err != nil {
		return fmt.Errorf("failed to read from storage: %w", err)
	}
	if group.Members == nil {
		group.Members = make(map[string]model.GSLBService)
	}
	group.Members[svc.ID] = svc

	return sr.store.Save(memberOf, group)
}

func (sr *ServiceGroupRepo) Delete(memberOf string) error {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	if err := sr.store.Delete(memberOf); err != nil {
		return fmt.Errorf("failed to delete servicegroup: %s: %w", memberOf, err)
	}
	return nil
}

func (sr *ServiceGroupRepo) DeleteMember(memberOf string, member model.GSLBService) error {
	sr.lock.Lock()
	defer sr.lock.Unlock()
	group, err := sr.store.Load(member.MemberOf)
	if err != nil {
		return fmt.Errorf("failed to fetch service group: %w", err)
	}

	delete(group.Members, member.ID)
	// store.Delete directly: sr.lock is already held, sr.Delete would re-lock and deadlock
	if len(group.Members) == 0 {
		if err := sr.store.Delete(member.MemberOf); err != nil {
			return fmt.Errorf("failed to delete servicegroup: %s: %w", member.MemberOf, err)
		}
		return nil
	}

	err = sr.store.Save(member.MemberOf, group)
	if err != nil {
		return fmt.Errorf("failed to delete member: %s in service group: %s: %w", member.ID, member.MemberOf, err)
	}

	return nil
}
