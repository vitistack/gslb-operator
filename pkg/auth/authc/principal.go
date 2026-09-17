package authc

import "context"

type AuthClass string

const (
	M2M AuthClass = "M2M" // machine authenticates itself
	C2M AuthClass = "C2M" // a client/human authenticates via an external IdP
)

// Principal is the verified identity behind a login.
// Roles/KeyVersion are server-side attributes stamped at authentication time,
// never self-claimed by the caller.
type Principal struct {
	Subject    string    `json:"sub"`
	Method     string    `json:"method"`
	Class      AuthClass `json:"class"`
	Roles      []string  `json:"roles,omitempty"`
	KeyVersion int       `json:"kv,omitempty"`
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
