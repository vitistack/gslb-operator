package service

import "github.com/vitistack/gslb-operator/internal/utils/timesutil"

type Service struct {
	Addr             string             `json:"address"`
	Datacenter       string             `json:"datacenter"`
	Interval         timesutil.Duration `json:"checkInterval"`
	FailureThreshold int                `json:"failureTreshold"`
	FailureCount     int
	check            func() error // TCP - half/full, HTTP(S)
}

func (s *Service) Execute() error {
	return s.check()
}

func (s *Service) OnFailure(err error) {
	s.FailureCount++
	if s.FailureCount%s.FailureThreshold == 0 { // threshold reached, service is considered down

	}
}

func (s *Service) SetCheck(check func() error) {
	s.check = check
}
