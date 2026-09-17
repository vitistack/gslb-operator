package routes

/*

import (
	"net/http"
)

const (
	ROOT = "/"

	AUTH            = ROOT + "auth"
	POST_AUTH_TOKEN = http.MethodPost + " " + AUTH + "/token"

	SPOOFS          = ROOT + "spoofs" // DNSDIST domain spoofs
	SPOOFS_HASH     = SPOOFS + "/hash"
	SPOOFS_ID       = SPOOFS + "/{fqdn}"
	GET_SPOOFS      = http.MethodGet + " " + SPOOFS // Route GET
	GET_SPOOFID     = http.MethodGet + " " + SPOOFS_ID
	GET_SPOOFS_HASH = http.MethodGet + " " + SPOOFS_HASH // Route to hash all spoofs, for config validation
	POST_SPOOF      = http.MethodPost + " " + SPOOFS     // Route POST

	OVERRIDE        = SPOOFS + "/override"                                       // override DNSDIST configuration
	GET_OVERRIDE    = http.MethodGet + " " + OVERRIDE + "/{" + MemberOf + "}"    // Route GET
	POST_OVERRIDE   = http.MethodPost + " " + OVERRIDE                           // Route POST
	DELETE_OVERRIDE = http.MethodDelete + " " + OVERRIDE + "/{" + MemberOf + "}" // Route DELETE

	METRICS     = ROOT + "metrics"
	GET_METRICS = http.MethodGet + " " + METRICS

	WEBHOOKS        = ROOT + "webhooks"
	GET_WEBHOOKS    = http.MethodGet + " " + WEBHOOKS
	POST_WEBHOOKS   = http.MethodPost + " " + WEBHOOKS
	PUT_WEBHOOKS    = http.MethodPut + " " + WEBHOOKS + "/{id}"
	DELETE_WEBHOOKS = http.MethodDelete + " " + WEBHOOKS + "/{id}"

	SERVICE            = ROOT + "service/{fqdn}"
	SERVICE_STATUS     = SERVICE + "/status"
	GET_SERVICE_STATUS = http.MethodGet + " " + SERVICE_STATUS
)

*/
const (
	MemberOf = "memberOf"
)
