package http

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// evidence: witnesses
func TestRetryable(t *testing.T) {
	status := func(code int) error {
		return &httputil.HTTPError{Status: code}
	}
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{"502", status(http.StatusBadGateway), true},
		{"503", status(http.StatusServiceUnavailable), true},
		{"504", status(http.StatusGatewayTimeout), true},
		// A 500 may come after Discord checked the credential.
		{"500", status(http.StatusInternalServerError), false},
		{"429", status(http.StatusTooManyRequests), false},
		{"400", status(http.StatusBadRequest), false},
		{"401", status(http.StatusUnauthorized), false},
		{"deadline", context.DeadlineExceeded, false},
		{"canceled", context.Canceled, false},
		{"other", errors.New("x"), false},
		{"nil", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Retryable(tt.err); got != tt.want {
				t.Fatalf("Retryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
