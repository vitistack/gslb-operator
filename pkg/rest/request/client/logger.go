package client

import "net/http"

type Logger interface {
	Debug(args ...any)
	Debugf(template string, args ...any)
	Info(args ...any)
	Infof(template string, args ...any)
	Warn(args ...any)
	Warnf(template string, args ...any)
	Error(args ...any)
	Errorf(template string, args ...any)
	Fatal(args ...any)
	Fatalf(template string, args ...any)
}

type LogInterception struct {
	Transport http.RoundTripper
	logger    Logger
}

func (li *LogInterception) RoundTrip(req *http.Request) (*http.Response, error) {
	trip := li.Transport
	if trip == nil {
		trip = http.DefaultTransport
	}

	li.logger.Debug("making %s request to: %s", req.Method, req.URL.String())
	resp, err := trip.RoundTrip(req)
	if err != nil {
		li.logger.Errorf("%s request to: %s failed: %s: status-code: %d", req.Method, req.URL.String(), err.Error(), resp.StatusCode)
	} else {
		li.logger.Debugf("%s request to: %s succeeded with status-code: %d", req.Method, req.URL.String(), resp.StatusCode)
	}

	return resp, err
}

func NewLogInterception(log Logger, base http.RoundTripper) http.RoundTripper {
	return &LogInterception{
		Transport: base,
		logger: log,
	}
}
