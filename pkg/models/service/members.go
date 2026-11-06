package service

import (
	"github.com/vitistack/gslb-operator/internal/utils/ip"
)

type Member struct {
	ID        string     `json:"id"`
	Priority  int        `json:"priority"`
	DC        string     `json:"datacenter"`
	Views     []string   `json:"views"`
	Address   ip.Address `json:"address"`
	IsHealthy bool       `json:"isHealthy"`
}
