package authz

import "context"

// Input is the question posed to the policy engine.
type Input struct {
	Subject  string            `json:"subject"`  // principal.Subject
	Method   string            `json:"method"`   // authc method that logged them in
	Class    string            `json:"class"`    // M2M / C2M
	Roles    []string          `json:"roles"`    // attributes; policy maps roles → perms
	Action   string            `json:"action"`   // HTTP verb or logical action
	Resource string            `json:"resource"` // route template, e.g. "/spoofs"
	Attrs    map[string]string `json:"attrs"`    // path params, tenant, etc.
}

// Decision is the engine's answer. Reason/Obligations are optional but let
// policies explain denials or attach constraints without changing this contract.
type Decision struct {
	Allow       bool     `json:"allow"`
	Reason      string   `json:"reason,omitempty"`
	Obligations []string `json:"obligations,omitempty"`
}

// Authorizer decides whether an Input is permitted.
type Authorizer interface {
	Authorize(ctx context.Context, in Input) (Decision, error)
}
