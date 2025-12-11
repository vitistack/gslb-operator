package model

import "github.com/vitistack/gslb-operator/internal/utils/timesutil"

// JSON - object in the TXT records for the GSLB - config zone
type GSLBConfig struct {
	Fqdn       string             `json:"fqdn"`
	Ip         string             `json:"ip"`
	Port       string             `json:"port"`
	Datacenter string             `json:"datacenter"`
	Interval   timesutil.Duration `json:"interval"`
	Priority   int                `json:"priority"`
	Type       string             `json:"check_type"`
}
