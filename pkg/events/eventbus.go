package events

import (
	"context"
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
