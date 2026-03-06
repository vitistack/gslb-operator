package events

import (
	"encoding/json"
	"fmt"
)

type FilterOption interface {
	Filter() EventFilter
}

type FilterOptionFactory func() FilterOption

var registry map[EventType]FilterOptionFactory

func Register(typ EventType, factory FilterOptionFactory) {
	registry[typ] = factory
}

func ResolveOptions(typ EventType, raw json.RawMessage) (FilterOption, error) {
	factory, ok := registry[typ]
	if !ok {
		return nil, fmt.Errorf("unsupported event type: %s", typ)
	}

	opts := factory()

	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, opts); err != nil {
			return nil, fmt.Errorf("invalid options for %s: %w", typ, err)
		}
	}

	return opts, nil
}
