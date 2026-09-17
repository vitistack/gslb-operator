// pkg/auth/authz/errors.go
package authz

import "errors"

var (
	ErrDenied       = errors.New("authz: access denied")
	ErrNoPrincipal  = errors.New("authz: no principal in context")
	ErrEngineFailed = errors.New("authz: policy engine error")
)
