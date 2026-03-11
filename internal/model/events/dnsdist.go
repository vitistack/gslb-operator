package events

import "github.com/vitistack/gslb-operator/pkg/models/spoofs"

// dnsdist:spoof:create
type DNSDistSpoofCreateEvent struct {
	Spoof spoofs.Spoof
}

// dnsdist:spoof:delete
type DNSDistSpoofDeleteEvent struct {
	Spoof spoofs.Spoof
}
