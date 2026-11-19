package gateway

import (
	"testing"

	discordgateway "github.com/seamanticus/dmwire/gateway"
	"github.com/seamanticus/dmwire/utils/ws"
)

func TestNew(t *testing.T) {
	t.Run("authentication failed", func(t *testing.T) {
		opts := New(discordgateway.DefaultIdentifier("token")).Opts()
		if !opts.ErrorIsFatalClose(&ws.CloseEvent{Code: 4004}) {
			t.Error("gateway reconnects after the token is rejected")
		}
	})
}
