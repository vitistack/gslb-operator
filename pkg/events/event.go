package events

import (
	"log/slog"
	"time"
)

type EventType string

type EventHandler interface {
	Handle(*Event)
	GetID() string
}

type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"event"`
	Payload   any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *Event) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", e.ID),
		slog.String("type", string(e.Type)),
		slog.Any("body", e.Payload),
		slog.Time("ts", e.Timestamp),
	)
}
