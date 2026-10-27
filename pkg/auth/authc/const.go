package authc

const (
	authPrefix = "gslb:auth:"

	authCPrefix    = authPrefix + "c:"
	replayPrefix   = authCPrefix + "jti:"
	registerPrefix = authCPrefix + "client:"

	authZPrefix  = authCPrefix + "z:"
	revokePrefix = authPrefix + "z:revoked:"
)
