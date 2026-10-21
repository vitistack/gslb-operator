package authc

import (
	"context"
	"errors"
)

// AttributeMaterializer projects a client's owned attribute values as the subset
// of its claimed values that fall within its authorized values, per an injected
// predicate. It has no notion of what the attributes mean.
type AttributeMaterializer struct {
	registry      *Registry
	authorizedKey string // coarse grant to filter against
	ownedKey      string // fine set to write
	within        func(authorized, claimed string) bool
}

func NewAttributeMaterializer(reg *Registry, authorizedKey, ownedKey string, within func(authorized, claimed string) bool) *AttributeMaterializer {
	return &AttributeMaterializer{
		registry:      reg,
		authorizedKey: authorizedKey,
		ownedKey:      ownedKey,
		within:        within,
	}
}

func (m *AttributeMaterializer) Materialize(ctx context.Context, claims map[string][]string) error {
	for clientID, claimed := range claims {
		attrs, err := m.registry.Attributes(ctx, clientID)
		if errors.Is(err, ErrClientNotFound) {
			continue // not enrolled / not eligible yet
		}

		if err != nil {
			return err
		}

		authorized := attrs[m.authorizedKey]
		owned := make([]string, 0, len(claimed))
		seen := make(map[string]struct{}, len(claimed))
		for _, v := range claimed {
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			if m.permits(authorized, v) {
				owned = append(owned, v)
			}
		}

		if err := m.registry.SetAttribute(ctx, clientID, m.ownedKey, owned); err != nil {
			return err
		}
	}

	return nil
}

func (m *AttributeMaterializer) permits(authorized []string, value string) bool {
	for _, a := range authorized {
		if m.within(a, value) {
			return true
		}
	}
	return false
}
