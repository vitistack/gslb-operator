package spoofs

import (
	"encoding/json"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/utils/ip"
)

type Spoof struct {
	Name    string     `json:"name"`
	UUID    string     `json:"uuid"`
	FQDN    string     `json:"fqdn"`
	Address ip.Address `json:"address"`
}

func (s *Spoof) UnmarshalJSON(b []byte) error {
	type Alias Spoof
	aux := struct {
		Address json.RawMessage `json:"address"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	addr, err := ip.ParseAddressJSON(aux.Address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}
	s.Address = addr

	return nil
}
