package update

import (
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
)

type UpdateError struct {
	err    error
	server string
	spoof  spoofs.Spoof
}

func (e UpdateError) Error() string {
	return fmt.Errorf("%s: %w", e.server, e.err).Error()
}
