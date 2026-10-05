package jauth

import (
	"errors"
	"fmt"
)

var (
	ErrMissingClientID    = errors.New("jauth: missing client ID")
	ErrMissingRedirectURL = errors.New("jauth: missing redirect URL")
	ErrInvalidBaseURL     = errors.New("jauth: invalid base URL")
	ErrInvalidRedirectURL = errors.New("jauth: invalid redirect URL")
	ErrMissingCode        = errors.New("jauth: missing authorization code")
	ErrMissingToken       = errors.New("jauth: missing access token")
	ErrInvalidUserInfo    = errors.New("jauth: invalid userinfo response")
	// ErrNotFound is retained for compatibility; the SDK does not use it.
	ErrNotFound = errors.New("jauth: not found")
)

// HTTPError describes a non-200 UserInfo response without exposing its body.
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string {
	return fmt.Sprintf("jauth: userinfo returned HTTP %d", e.StatusCode)
}
