package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// RetryDelay is how many seconds an authentication request whose connection failed waits before its one retry.
const RetryDelay = 5

// Retryable reports whether err means the request did not reach Discord or a proxy in front of it failed, so sending it again is not a second guess at a credential.
func Retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if httpErr, ok := errors.AsType[*httputil.HTTPError](err); ok {
		switch httpErr.Status {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	_, ok := errors.AsType[httputil.RequestError](err)
	return ok
}
