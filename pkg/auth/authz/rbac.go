package authz

import (
	"context"
	"strings"
	"sync/atomic"
)

type RBACAuthorizer struct {
	compiled   atomic.Pointer[roleGrants]
	scopeGuard ScopeGuard
}

func NewRBACAuthorizer(p Policy, guard ScopeGuard) *RBACAuthorizer {
	a := &RBACAuthorizer{scopeGuard: guard}
	a.compiled.Store(compile(p))
	return a
}

func (a *RBACAuthorizer) Reload(p Policy) { a.compiled.Store(compile(p)) }

func (a *RBACAuthorizer) Authorize(ctx context.Context, input Input) (Decision, error) {
	rg := a.compiled.Load()

	scopeDenied := false
	for _, role := range input.Roles {
		role, ok := rg.roles[role]
		if !ok || !role.grants(input.Action) {
			continue
		}

		if role.scope != ScopeOwned {
			return Decision{Allow: true}, nil
		}

		permitted, err := a.permit(ctx, input)
		if err != nil {
			return Decision{}, err
		}

		if permitted {
			return Decision{Allow: true}, nil
		}

		scopeDenied = true
	}

	if scopeDenied {
		return Decision{Allow: false, Reason: "subject does not own the target resource"}, nil
	}

	return Decision{Allow: false, Reason: "no role grants action " + input.Action}, nil
}

// owns fails closed: without a guard, ownership cannot be proven, so deny.
func (a *RBACAuthorizer) permit(ctx context.Context, in Input) (bool, error) {
	if a.scopeGuard == nil {
		return false, nil
	}
	return a.scopeGuard.Permit(ctx, in)
}

type roleGrants struct {
	roles map[string]compiledRole
}

type compiledRole struct {
	matcher matcher
	scope   Scope
}

func (r compiledRole) grants(action string) bool { return r.matcher.grants(action) }

type matcher struct {
	all      bool
	exact    map[string]struct{}
	prefixes []string // "spoofs." matches "spoofs.read"
}

func (m matcher) grants(action string) bool {
	if m.all {
		return true
	}

	if _, ok := m.exact[action]; ok {
		return true
	}

	for _, p := range m.prefixes {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	return false
}

func compile(p Policy) *roleGrants {
	rg := &roleGrants{roles: make(map[string]compiledRole, len(p.Roles))}
	resolved, err := p.resolve()
	if err != nil {
		return rg // fail closed; Validate surfaces the error loudly before serving/reload
	}

	for role, rr := range resolved {
		m := matcher{exact: map[string]struct{}{}}
		for _, pat := range rr.actions {
			switch {
			case pat == "*":
				m.all = true
			case strings.HasSuffix(pat, ".*"):
				m.prefixes = append(m.prefixes, strings.TrimSuffix(pat, "*"))
			default:
				m.exact[pat] = struct{}{}
			}
		}
		rg.roles[role] = compiledRole{matcher: m, scope: rr.scope}
	}

	return rg
}
