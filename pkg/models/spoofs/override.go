package spoofs

import (
	"encoding/json"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/utils/ip"
)

type Override struct {
	MemberOf string     `json:"memberOf"`
	Address  ip.Address `json:"address"`
}

func (o *Override) UnmarshalJSON(b []byte) error {
	type Alias Override
	aux := struct {
		Address json.RawMessage `json:"address"`
		*Alias
	}{
		Alias: (*Alias)(o),
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	addr, err := ip.ParseAddressJSON(aux.Address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}
	o.Address = addr

	return nil
}
