package service

import (
	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
)

type Members struct {
	pagination.Pagination[Member]
}

type Member struct {
	ID        string     `json:"id"`
	Priority  int        `json:"priority"`
	DC        string     `json:"datacenter"`
	Views     []string   `json:"views"`
	Address   ip.Address `json:"address"`
	IsHealthy bool       `json:"isHealthy"`
}
