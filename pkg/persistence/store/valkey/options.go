package valkey

import "github.com/vitistack/gslb-operator/pkg/persistence"

type Option[T any] func(*ValkeyStore[T])

func WithMigrations[T any](fns ...persistence.MigrateFunc) Option[T] {
	return func(s *ValkeyStore[T]) {
		s.migrate = persistence.Chain(fns...)
	}
}
