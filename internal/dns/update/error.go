package update

import (
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type UpdateError struct {
	Err    error
	Server string
	Spoof  spoofs.Spoof
}

func (e UpdateError) Error() string {
	return fmt.Errorf("%s: %w", e.Server, e.Err).Error()
}
