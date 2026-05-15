package manager

import (
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
)

type managerConfig struct {
	MinRunningWorkers     uint
	NonBlockingBufferSize uint
	DryRun                bool
	repo                  *servicegroup.ServiceGroupRepo
}

type serviceManagerOption func(cfg *managerConfig)

func WithMinRunningWorkers(workers uint) serviceManagerOption {
	return func(cfg *managerConfig) {
		cfg.MinRunningWorkers = workers
	}
}

func WithNonBlockingBufferSize(bufferSize uint) serviceManagerOption {
	return func(cfg *managerConfig) {
		cfg.NonBlockingBufferSize = bufferSize
	}
}

func WithDryRun(enabled bool) serviceManagerOption {
	return func(cfg *managerConfig) {
		cfg.DryRun = enabled
	}
}

func WithServiceGroupRepository(repo *servicegroup.ServiceGroupRepo) serviceManagerOption {
	return func(cfg *managerConfig) {
		cfg.repo = repo
	}
}
