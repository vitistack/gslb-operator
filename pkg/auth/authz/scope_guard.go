package authz

import (
	"context"
	"slices"
)

// AttributeSource returns the attribute values a subject is authorized for.
type AttributeSource interface {
	Attributes(ctx context.Context, subject string) (map[string][]string, error)
}

// ScopeGuard decides whether the Input's subject owns the resource the
// request targets, for ScopeOwned roles.
type ScopeGuard interface {
	Permit(ctx context.Context, in Input) (bool, error)
}

type allOf []ScopeGuard

func NewScopeGuard(guards ...ScopeGuard) ScopeGuard {
	return allOf(guards)
}

func (a allOf) Permit(ctx context.Context, in Input) (bool, error) {
	for _, g := range a {
		ok, err := g.Permit(ctx, in)
		if err != nil || !ok {
			return false, err
		}
	}
	return len(a) > 0, nil
}

// MultiAttributeScopeGuard permits a scoped grant when every configured key
// present in the request has a value among the subject's authorized values.
// Keys absent from the request are skipped; a request touching none is denied.
type MultiAttributeScopeGuard struct {
    source AttributeSource
    keys   []string
}

func NewMultiAttributeScopeGuard(source AttributeSource, keys ...string) *MultiAttributeScopeGuard {
    return &MultiAttributeScopeGuard{source: source, keys: keys}
}

func (g *MultiAttributeScopeGuard) Permit(ctx context.Context, in Input) (bool, error) {
    owned, err := g.source.Attributes(ctx, in.Subject)
    if err != nil {
        return false, err
    }

    checked := 0
    for _, key := range g.keys {
        target := in.Attrs[key]
        if target == "" {
            continue // this dimension isn't part of the request
        }
        checked++
        if !contains(owned[key], target) {
            return false, nil // present but not owned → deny
        }
    }
    if checked == 0 {
        return false, nil // fail closed: nothing to scope against
    }
    return true, nil
}

func contains(vals []string, target string) bool {
    return slices.Contains(vals, target)
}

// AttributeScopeGuard permits a scoped grant when the request's value for the
// configured attribute key is among the subject's authorized values for it.
type AttributeScopeGuard struct {
	source AttributeSource
	key    string // request attribute key, e.g. "memberOf"
}

func NewAttributeScopeGuard(source AttributeSource, key string) *AttributeScopeGuard {
	return &AttributeScopeGuard{source: source, key: key}
}

func (g *AttributeScopeGuard) Permit(ctx context.Context, in Input) (bool, error) {
	target := in.Attrs[g.key]
	if target == "" {
		return false, nil // nothing to scope against → fail closed
	}

	owned, err := g.source.Attributes(ctx, in.Subject)
	if err != nil {
		return false, err
	}
	if slices.Contains(owned[g.key], target) {
		return true, nil
	}

	return false, nil
}
