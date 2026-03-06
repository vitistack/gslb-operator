package events

import "github.com/vitistack/gslb-operator/internal/model"

// gslb:failover
type GSLBFailoverEvent struct {
	MemberOf   string
	LastActive model.GSLBService
	NewActive  model.GSLBService
}

// gslb:member:healthchange
type GSLBMemberHealthChangeEvent struct {
	Member model.GSLBService
}

// gslb:service:up
type GSLBServiceUpEvent struct {
	NewActive model.GSLBService
}

// gslb:service:down
type GSLBServiceDownEvent struct {
	MemberOf string
}

// gslb:config:create
type GSLBConfigCreateEvent struct {
	Config model.GSLBConfig
}

// gslb:config:update
type GSLBConfigUpdateEvent struct {
	LastConfig model.GSLBConfig
	Current    model.GSLBConfig
}

// gslb:config:delete
type GSLBConfigDeleteEvent struct {
	LastConfig model.GSLBConfig
}
