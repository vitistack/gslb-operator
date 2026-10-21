package authc

import "errors"

var (
	ErrActiveToken               = errors.New("authc: client already has an active token")
	ErrUnauthorized              = errors.New("authc: unauthorized")
	ErrClientExists              = errors.New("authc: client_id already enrolled")
	ErrClientNotFound            = errors.New("authc: client not enrolled")
	ErrKeyMismatch               = errors.New("authc: public key mismatch")
	ErrReplayed                  = errors.New("authc: assertion replayed")
	ErrUnknownMethod             = errors.New("authc: unknown authentication method")
	ErrNoMethodDetected          = errors.New("authc: could not detect an authentication method")
	ErrTooLongClientAssertionTTL = errors.New("authc: client-assertion token exceeds tolerated TTL")
)
