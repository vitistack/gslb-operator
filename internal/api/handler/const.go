package handler

import "net/http"

const (
	ROOT = "/"

	SPOOFS          = ROOT + "spoofs"               // DNSDIST domain spoofs
	GET_SPOOFS      = http.MethodGet + " " + SPOOFS // Route GET
	GET_SPOOFID     = GET_SPOOFS + "/{fqdn}"
	GET_SPOOFS_HASH = GET_SPOOFS + "/hash"           // Route to hash all spoofs, for config validation
	POST_SPOOF      = http.MethodPost + " " + SPOOFS // Route POST

	OVERRIDE        = ROOT + "overrides"                 // override DNSDIST configuration
	GET_OVERRIDES   = http.MethodGet + " " + OVERRIDE    // Route GET
	POST_OVERRIDE   = http.MethodPost + " " + OVERRIDE   // Route POST
	DELETE_OVERRIDE = http.MethodDelete + " " + OVERRIDE // Route DELETE
)
