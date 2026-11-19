package gateway

import (
	"testing"

	discordgateway "github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/http"
)

func TestNew(t *testing.T) {
	t.Run("authentication failed", func(t *testing.T) {
		opts := New(http.Network{}, discordgateway.DefaultIdentifier("token")).Opts()
		if !opts.ErrorIsFatalClose(&ws.CloseEvent{Code: 4004}) {
			t.Error("gateway reconnects after the token is rejected")
		}
	})
}
