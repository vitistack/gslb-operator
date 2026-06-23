package update

import "github.com/vitistack/gslb-operator/internal/utils/ip"

type Record struct {
	Name    string
	Address ip.Address
	UUID    string
	View    string
}
