package checks

import (
	"errors"
	"math/rand"
)

type DryRun struct{}

func (dr *DryRun) Check() error {
	num := rand.Intn(10)
	if num == 0 { // 10% failure when dryrunning
		return errors.New("dry-run fail")
	}
	return nil
}
