package service

type ServiceOption func(s *Service)

func WithDryRunChecks(enabled bool) ServiceOption {
	return func(s *Service) {
		s.dryRun = enabled
	}
}

func WithHealthy() ServiceOption {
	return func(s *Service) {
		s.isHealthy = true
	}
}

func WithFailureCount(count int) ServiceOption {
	return func(s *Service) {
		if count > -1 {
			s.failureCount = count
		} // default values are handled in the creation of the service!
	}
}
