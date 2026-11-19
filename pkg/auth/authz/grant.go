package authz

import (
	"context"
	"slices"
)

// Grant is what an authorized request may touch; the zero value permits nothing.
type Grant struct {
	allowed     bool
	constraints map[string][]string // nil means unrestricted
}

func (g Grant) Unrestricted() bool { return g.allowed && g.constraints == nil }

// Permits reports whether a resource whose attribute key has value is covered.
func (g Grant) Permits(key, value string) bool {
	if !g.allowed {
		return false
	}
	if g.constraints == nil {
		return true
	}
	return slices.Contains(g.constraints[key], value)
}

func (g Grant) Filter[T any](key string, attr func(T) string) func(T) bool {
	if g.Unrestricted() {
		return func(T) bool { return true }
	}
	return func(v T) bool { return g.Permits(key, attr(v)) }
}

type grantKey struct{}

func withGrant(ctx context.Context, g Grant) context.Context {
	return context.WithValue(ctx, grantKey{}, g)
}

func GrantFrom(ctx context.Context) (Grant, bool) {
	g, ok := ctx.Value(grantKey{}).(Grant)
	return g, ok
}
