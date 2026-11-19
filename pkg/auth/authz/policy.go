package authz

import (
	"fmt"
	"slices"
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

// Role grants every listed action on every object type matched by Paths.
type Role struct {
	Inherits   []string          `yaml:"inherits,omitempty"` // union this role's grants with the named roles
	Paths      []string          `yaml:"paths,omitempty"`    // object types: exact, "*", or "prefix/*"
	Actions    []string          `yaml:"actions,omitempty"`  // read, write, update, delete
	Scope      Scope             `yaml:"scope,omitempty"`
	Attributes map[string]string `yaml:"attributes,omitempty"` // reserved for further ABAC
}

// permission stays intact across inheritance so a child's actions never pair with a parent's paths.
type permission struct {
	paths   []string
	actions []string
}

func (p permission) grants(objectType, action string) bool {
	return slices.Contains(p.actions, action) &&
		slices.ContainsFunc(p.paths, func(pat string) bool { return pathMatches(pat, objectType) })
}

// resolvedRole is a role flattened across its inheritance chain.
type resolvedRole struct {
	permissions []permission
	scope       Scope
}

func (r resolvedRole) grants(objectType, action string) bool {
	return slices.ContainsFunc(r.permissions, func(p permission) bool { return p.grants(objectType, action) })
}

func pathMatches(pattern, objectType string) bool {
	switch {
	case pattern == "*":
		return true
	case strings.HasSuffix(pattern, "/*"):
		return strings.HasPrefix(objectType, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == objectType
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

// resolve flattens every role across its inheritance chain, unioning
// permissions and taking the most-restrictive scope (owned wins) so
// inheritance can never widen a scoped ancestor. It errors on a cycle or an
// unknown parent.
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

		var perms []permission
		if len(role.Paths) > 0 && len(role.Actions) > 0 {
			perms = append(perms, permission{paths: role.Paths, actions: role.Actions})
		}
		scope := role.Scope
		for _, parent := range role.Inherits {
			pr, err := one(parent)
			if err != nil {
				return resolvedRole{}, err
			}

			perms = append(perms, pr.permissions...)
			if pr.scope == ScopeOwned {
				scope = ScopeOwned
			}
		}

		visiting[roleName] = false
		rr := resolvedRole{permissions: perms, scope: scope}
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

// Validate rejects inheritance cycles, unknown parents, unknown scopes,
// half-defined grants, and actions outside read/write/update/delete.
func (p Policy) Validate() error {
	if _, err := p.resolve(); err != nil {
		return err
	}
	for name, r := range p.Roles {
		switch r.Scope {
		case ScopeAll, ScopeOwned:
		default:
			return fmt.Errorf("role %q has unknown scope %q", name, r.Scope)
		}
		if (len(r.Paths) == 0) != (len(r.Actions) == 0) {
			return fmt.Errorf("role %q must set both paths and actions", name)
		}
		for _, a := range r.Actions {
			if !slices.Contains(knownActions, a) {
				return fmt.Errorf("role %q grants unknown action %q", name, a)
			}
		}
	}
	return nil
}

// ValidatePaths rejects role paths that match none of the routed object types.
func (p Policy) ValidatePaths(objectTypes []string) error {
	for name, r := range p.Roles {
		for _, pat := range r.Paths {
			if !slices.ContainsFunc(objectTypes, func(ot string) bool { return pathMatches(pat, ot) }) {
				return fmt.Errorf("role %q grants unknown path %q", name, pat)
			}
		}
	}
	return nil
}
