package events

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"
)

var eventBus *EventBus

func init() {
	eventBus = NewBus()
}

func On(typ EventType, handler EventHandler, filters ...EventFilter) {
	eventBus.On(typ, handler, filters...)
}

func Emit(events ...*Event) {
	eventBus.Emit(events...)
}

// returns the full tree of an event type, with a custom delimiter
// default is ':'
func Tree(typ EventType, delimiter ...rune) []EventType {
	s := string(typ)
	sep := ":"
	if len(delimiter) > 0 {
		sep = string(delimiter[0])
	}

	parts := strings.Split(s, sep)
	types := make([]EventType, len(parts))
	for i := range parts {
		types[i] = EventType(strings.Join(parts[:i+1], sep))
	}

	// so the bottom-level types are at the top/first in the array
	slices.Reverse(types)

	return types
}

func Remove(typ EventType, id string) {
	eventBus.Remove(typ, id)
}

func Stop(ctx context.Context) {
	eventBus.Stop(ctx)
}

type EventBus struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	wg       sync.WaitGroup
	sem      Semaphore
}

func NewBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
		mu:       sync.RWMutex{},
		sem:      NewSemaphore(10),
	}
}

func (eb *EventBus) On(typ EventType, handler EventHandler, filters ...EventFilter) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for _, subTyp := range Tree(typ) {
		eb.handlers[subTyp] = append(
			eb.handlers[subTyp],
			&Subscription{
				handler: handler,
				filters: filters,
			},
		)
	}
}

func (eb *EventBus) Emit(events ...*Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, event := range events {
		eb.dispatch(event)
	}
}

func (eb *EventBus) dispatch(event *Event) {
	seen := make(map[string]struct{})
	for _, subType := range Tree(event.Type) {
		for _, handler := range eb.handlers[subType] {
			id := handler.GetID()
			if _, ok := seen[id]; ok {
				continue
			}

			seen[id] = struct{}{}
			eb.wg.Go(func() {
				eb.sem.Aquire()
				defer eb.sem.Release()

				handler.Handle(event)
				log.Println(subType)
			})
		}
	}
}

func (eb *EventBus) Remove(typ EventType, id string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	handlers, ok := eb.handlers[typ]
	if !ok {
		return
	}

	idx := slices.IndexFunc(handlers, func(e EventHandler) bool {
		return e.GetID() == id
	})

	if idx != -1 {
		eb.handlers[typ] = append(handlers[:idx], handlers[idx+1:]...)
	}
}

func (eb *EventBus) Stop(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		eb.wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		return
	}
}
