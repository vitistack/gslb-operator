package valkey

import (
	"time"

	"github.com/vitistack/gslb-operator/pkg/persistence"
)

type Option[T any] func(*ValkeyStore[T])

func WithMigrations[T any](fns ...persistence.MigrateFunc) Option[T] {
	return func(s *ValkeyStore[T]) {
		s.migrate = persistence.Chain(fns...)
	}
}

func WithOpTimeout[T any](timeout time.Duration) Option[T] {
	return func(vs *ValkeyStore[T]) {
		vs.opTimeout = timeout
	}
}
