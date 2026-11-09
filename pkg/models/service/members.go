package service

import (
	"encoding/json"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
)

type GetMembersParams struct {
	Views     string `param:"views"`
	IsHealthy *bool  `param:"isHealthy"`
	pagination.PaginationParams
}

type Member struct {
	ID        string     `json:"id"`
	Priority  int        `json:"priority"`
	DC        string     `json:"datacenter"`
	Views     []string   `json:"views"`
	Address   ip.Address `json:"address"`
	IsHealthy bool       `json:"isHealthy"`
}

func (m *Member) UnmarshalJSON(b []byte) error {
	type Alias Member
	aux := struct {
		Address json.RawMessage `json:"address"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	addr, err := ip.ParseAddressJSON(aux.Address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}

	if addr == nil {
		return fmt.Errorf("invalid GSLB-config: address field cannot be a nil value")
	}

	m.Address = addr

	return nil
}
