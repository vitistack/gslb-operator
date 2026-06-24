package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

// JSON - object in the TXT records for the GSLB - config zone
type GSLBConfig struct {
	ServiceID        string             `json:"id"`
	Fqdn             string             `json:"fqdn"`
	MemberOf         string             `json:"memberOf"`
	Address          ip.Address         `json:"address"`
	Port             string             `json:"port"`
	Datacenter       string             `json:"dc"`
	Interval         timesutil.Duration `json:"interval"`
	Priority         int                `json:"priority"`
	FailureThreshold int                `json:"threshold"`
	CheckType        string             `json:"check"`
	Script           *LuaScript         `json:"lua,omitempty"`
}

func (c *GSLBConfig) UnmarshalJSON(b []byte) error {
	type Alias GSLBConfig
	aux := struct {
		Address json.RawMessage `json:"address"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}

	addr, err := ip.ParseAddressJSON(aux.Address)
	if err != nil {
		return fmt.Errorf("address: %w", err)
	}
	c.Address = addr

	return nil
}

type LuaScript string

func (s *LuaScript) UnmarshalJSON(b []byte) error {
	var encoded string
	if err := json.Unmarshal(b, &encoded); err != nil {
		*s = LuaScript("")
		return err
	}

	if encoded == "" {
		*s = LuaScript("")
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		*s = LuaScript("")
		return fmt.Errorf("failed to decode base64 lua script: %w", err)
	}

	*s = LuaScript(decoded)

	return nil
}
