package proxy

import "errors"

var (
	// ErrInvalidInput indicates the caller supplied invalid parameters.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUpstream indicates a failure while interacting with external systems.
	ErrUpstream = errors.New("upstream failure")
)
