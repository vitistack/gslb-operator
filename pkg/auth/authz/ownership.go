package authz

import (
	"context"
	"errors"
	"slices"

	"github.com/vitistack/gslb-operator/pkg/auth/authc"
)

// AttributeSource returns the attribute values a subject is authorized for.
type AttributeSource interface {
	Attributes(ctx context.Context, subject string) (map[string][]string, error)
}

// OwnershipAuthorizer fulfils the owned obligation by constraining the decision
// to the subject's owned attribute values.
type OwnershipAuthorizer struct {
	next          Authorizer
	source        AttributeSource
	attributeKeys []string
}

func NewOwnershipAuthorizer(next Authorizer, source AttributeSource, keys ...string) *OwnershipAuthorizer {
	return &OwnershipAuthorizer{next: next, source: source, attributeKeys: keys}
}

func (a *OwnershipAuthorizer) Authorize(ctx context.Context, in Input) (Decision, error) {
	dec, err := a.next.Authorize(ctx, in)
	if err != nil || !dec.Allow || !slices.Contains(dec.Obligations, string(ScopeOwned)) {
		return dec, err
	}

	owned, err := a.source.Attributes(ctx, in.Subject)
	if err != nil && !errors.Is(err, authc.ErrClientNotFound) {
		return Decision{}, err
	}

	for _, key := range a.attributeKeys {
		if target, ok := in.Attrs[key]; ok && !slices.Contains(owned[key], target) {
			return Decision{Allow: false, Reason: key + " " + target + " not owned by subject"}, nil
		}
	}

	constraints := make(map[string][]string, len(owned)) // non-nil: unlisted attributeKeys permit nothing
	for key, vals := range owned {
		constraints[key] = slices.Clone(vals)
	}

	dec.Constraints = constraints
	dec.Obligations = slices.DeleteFunc(slices.Clone(dec.Obligations), func(o string) bool { return o == string(ScopeOwned) })
	return dec, nil
}
