package model

import "github.com/vitistack/gslb-operator/internal/utils/timesutil"

// JSON - object in the TXT records for the GSLB - config zone
type GSLBConfig struct {
	Address    string             `json:"address"`
	Datacenter string             `json:"datacenter"`
	Interval   timesutil.Duration `json:"interval"`
	Priority   int                `json:"priority"`
	Type       string             `json:"type"`
}
