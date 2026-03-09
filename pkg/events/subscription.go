package events

// returns wether an event should be handled or not
type EventFilter func(*Event) bool

type Subscription struct {
	handler EventHandler
	filters []EventFilter
}

func (s *Subscription) Handle(e *Event) {
	if s.filters != nil {
		for _, filter := range s.filters {
			if !filter(e) {
				return
			}
		}
	}

	s.handler.Handle(e)
}

func (s *Subscription) GetID() string {
	return s.handler.GetID()
}
