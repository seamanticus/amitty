package gateway

import (
	"testing"

	"github.com/gorilla/websocket"
	"github.com/seamanticus/amitty/internal/http"
)

func TestNewDialer(t *testing.T) {
	dialer := NewDialer(http.Network{})
	t.Run("spoofed TLS", func(t *testing.T) {
		if dialer.NetDialTLSContext == nil {
			t.Error("TLS dialer is nil")
		}
	})
	t.Run("no proxy", func(t *testing.T) {
		if dialer.Proxy != nil {
			t.Error("proxy is enabled")
		}
	})
	t.Run("compression", func(t *testing.T) {
		if !dialer.EnableCompression {
			t.Error("compression is disabled")
		}
	})
	t.Run("default dialer unchanged", func(t *testing.T) {
		if websocket.DefaultDialer.NetDialTLSContext != nil {
			t.Error("default dialer was modified")
		}
	})
}
