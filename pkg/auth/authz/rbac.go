package authz

import (
	"context"
	"sync/atomic"
)

type RBACAuthorizer struct {
	compiled atomic.Pointer[roleGrants]
}

func NewRBACAuthorizer(p Policy) *RBACAuthorizer {
	a := &RBACAuthorizer{}
	a.compiled.Store(compile(p))
	return a
}

func (a *RBACAuthorizer) Reload(p Policy) { a.compiled.Store(compile(p)) }

func (a *RBACAuthorizer) Authorize(_ context.Context, in Input) (Decision, error) {
	rg := a.compiled.Load()

	owned := false
	for _, name := range in.Roles {
		role, ok := rg.roles[name]
		if !ok || !role.grants(in.Resource, in.Action) {
			continue
		}

		if role.scope != ScopeOwned {
			return Decision{Allow: true}, nil
		}
		owned = true
	}

	if owned {
		return Decision{Allow: true, Obligations: []string{string(ScopeOwned)}}, nil
	}

	return Decision{Allow: false, Reason: "no role grants " + in.Action + " on " + in.Resource}, nil
}

type roleGrants struct {
	roles map[string]resolvedRole
}

func compile(p Policy) *roleGrants {
	resolved, err := p.resolve()
	if err != nil {
		return &roleGrants{} // fail closed; Validate surfaces the error loudly before serving/reload
	}
	return &roleGrants{roles: resolved}
}
