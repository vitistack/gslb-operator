package checks

import (
	"errors"
	"math/rand"
)

// In-use in development and testing to simulate a health-check.
func DryRun() func() error {
	return func() error {
		num := rand.Intn(10)
		if num == 0 { // 10% failure when dryrunning
			return errors.New("dry-run fail")
		}
		return nil
	}
}
