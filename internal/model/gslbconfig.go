package model

import "github.com/vitistack/gslb-operator/internal/utils/timesutil"

// JSON - object in the TXT records for the GSLB - config zone
type GSLBConfig struct {
	ServiceID        string             `json:"service_id"`
	Fqdn             string             `json:"fqdn"`
	MemberOf         string             `json:"memberOf"`
	Ip               string             `json:"ip"`
	Port             string             `json:"port"`
	Datacenter       string             `json:"datacenter"`
	Interval         timesutil.Duration `json:"interval"`
	Priority         int                `json:"priority"`
	FailureThreshold int                `json:"failure_threshold"`
	CheckType        string             `json:"check_type"`
	Script           string             `json:"lua"`
}
