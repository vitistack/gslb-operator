package events

import (
	"context"
	"slices"
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
}

func NewBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
		mu:       sync.RWMutex{},
	}
}

func (eb *EventBus) On(typ EventType, handler EventHandler, filters ...EventFilter) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers[typ] = append(
		eb.handlers[typ],
		&Subscription{
			handler: handler,
			filters: filters,
		},
	)
}

func (eb *EventBus) Emit(events ...*Event) {
	for _, event := range events {
		for _, handler := range eb.handlers[event.Type] {
			eb.wg.Go(func() {
				handler.Handle(event)
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
		handlers = append(handlers[:idx], handlers[idx+1:]...)
		eb.handlers[typ] = handlers
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
