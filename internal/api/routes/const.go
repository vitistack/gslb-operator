package routes

import "net/http"

const (
	ROOT = "/"

	SPOOFS          = ROOT + "spoofs" // DNSDIST domain spoofs
	SPOOFS_HASH     = SPOOFS + "/hash"
	SPOOFS_ID       = SPOOFS + "/{fqdn}"
	GET_SPOOFS      = http.MethodGet + " " + SPOOFS // Route GET
	GET_SPOOFID     = http.MethodGet + " " + SPOOFS_ID
	GET_SPOOFS_HASH = http.MethodGet + " " + SPOOFS_HASH // Route to hash all spoofs, for config validation
	POST_SPOOF      = http.MethodPost + " " + SPOOFS     // Route POST

	OVERRIDE        = SPOOFS + "/override"                        // override DNSDIST configuration
	GET_OVERRIDE    = http.MethodGet + " " + OVERRIDE + "/{fqdn}" // Route GET
	POST_OVERRIDE   = http.MethodPost + " " + OVERRIDE            // Route POST
	DELETE_OVERRIDE = http.MethodDelete + " " + OVERRIDE          // Route DELETE

	FAILOVER      = ROOT + "failover"
	POST_FAILOVER = http.MethodGet + " " + FAILOVER

	AUTH            = ROOT + "auth"
	AUTH_LOGIN      = AUTH + "/login"
	POST_AUTH_LOGIN = http.MethodPost + " " + AUTH_LOGIN
)
