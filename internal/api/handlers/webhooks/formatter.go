package webhooks

import "github.com/vitistack/gslb-operator/pkg/events"

// formats the event into an accepted format at webhook endpoint
// therefore webhooks only supported for specific types
type Formatter interface {
	Format(*events.Event) ([]byte, error)
}
