package service

import "errors"


var (
	ErrUnableToParseIpAddr = errors.New("unable to parse ip address")
	ErrUnableToResolveAddr = errors.New("unable to resolve address")
)