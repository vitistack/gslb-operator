package authz

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type Policy struct {
	Roles map[string]Role `yaml:"roles"`
}

// Scope narrows a role's grants to the resources the subject owns.
type Scope string

const (
	ScopeAll   Scope = ""      // default: grants apply to any instance
	ScopeOwned Scope = "owned" // grants apply only to the subject's owned memberOf
)

type Role struct {
	Inherits   []string          `yaml:"inherits,omitempty"` // union this role's grants with the named roles
	Actions    []string          `yaml:"actions"`
	Scope      Scope             `yaml:"scope,omitempty"`
	Attributes map[string]string `yaml:"attributes,omitempty"` // reserved for further ABAC
}

// resolvedRole is a role flattened across its inheritance chain.
type resolvedRole struct {
	actions []string
	scope   Scope
}

func ParsePolicy(data []byte) (Policy, error) {
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	if len(p.Roles) == 0 {
		return Policy{}, fmt.Errorf("policy defines no roles")
	}
	return p, nil
}

// resolve flattens every role across its inheritance chain, unioning actions
// and taking the most-restrictive scope (owned wins) so inheritance can never
// widen a scoped ancestor. It errors on a cycle or an unknown parent.
func (p Policy) resolve() (map[string]resolvedRole, error) {
	out := make(map[string]resolvedRole, len(p.Roles))
	visiting := make(map[string]bool)

	var one func(roleName string) (resolvedRole, error)
	one = func(roleName string) (resolvedRole, error) {
		if rr, done := out[roleName]; done {
			return rr, nil
		}

		role, ok := p.Roles[roleName]
		if !ok {
			return resolvedRole{}, fmt.Errorf("inherits unknown role %q", roleName)
		}

		if visiting[roleName] {
			return resolvedRole{}, fmt.Errorf("inheritance cycle through %q", roleName)
		}
		visiting[roleName] = true

		actions := append([]string{}, role.Actions...)
		scope := role.Scope
		for _, parent := range role.Inherits {
			pr, err := one(parent)
			if err != nil {
				return resolvedRole{}, err
			}

			actions = append(actions, pr.actions...)
			if pr.scope == ScopeOwned {
				scope = ScopeOwned
			}
		}

		visiting[roleName] = false
		rr := resolvedRole{actions: actions, scope: scope}
		out[roleName] = rr
		return rr, nil
	}

	for roleName := range p.Roles {
		if _, err := one(roleName); err != nil {
			return nil, fmt.Errorf("role %q: %w", roleName, err)
		}
	}
	return out, nil
}

// Validate rejects a policy with inheritance cycles, unknown parents, unknown
// scopes, or actions absent from the inventory. Wildcards are exempt.
func (p Policy) Validate(known []Action) error {
	if _, err := p.resolve(); err != nil {
		return err
	}
	set := make(map[string]struct{}, len(known))
	for _, a := range known {
		set[string(a)] = struct{}{}
	}
	for role, r := range p.Roles {
		switch r.Scope {
		case ScopeAll, ScopeOwned:
		default:
			return fmt.Errorf("role %q has unknown scope %q", role, r.Scope)
		}
		for _, pat := range r.Actions {
			if pat == "*" || strings.HasSuffix(pat, ".*") {
				continue
			}
			if _, ok := set[pat]; !ok {
				return fmt.Errorf("role %q grants unknown action %q", role, pat)
			}
		}
	}
	return nil
}
