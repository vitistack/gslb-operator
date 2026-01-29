package client

import "net/http"

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
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

	li.logger.Debug("making request", "method", req.Method, "endpoint", req.URL.String())
	resp, err := trip.RoundTrip(req)
	if err != nil {
		li.logger.Error("request failed", "reason", err.Error(), "method", req.Method, "endpoint", req.URL.String())
	} else {
		li.logger.Debug("request succeeded", "status_code", resp.StatusCode, "endpoint", req.URL.String())
	}

	return resp, err
}

func NewLogInterception(log Logger, base http.RoundTripper) http.RoundTripper {
	return &LogInterception{
		Transport: base,
		logger:    log,
	}
}
