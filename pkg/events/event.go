package events

import "time"

type EventType string

type EventHandler interface {
	Handle(*Event)
	GetID() string
}

type Event struct {
	Type      EventType `json:"event"`
	Payload   any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}
