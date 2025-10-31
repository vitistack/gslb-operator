package service

import (
	"time"
)

type Service struct {
	Addr             string
	Datacenter       string
	Interval         time.Duration
	FailureCount     int
	FailureThreshold int
	check            func() error // TCP - half/full, HTTP(S)
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Execute() error {
	return s.check()
}

func (s *Service) OnFailure(err error) {
	s.FailureCount++
	if s.FailureCount%s.FailureThreshold == 0 { // threshold reached, service is considered down

	}
}
