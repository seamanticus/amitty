package gateway

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
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

// The headers the web client's handshake carries in a NetLog of Brave 157, apart from its cookies and Brave's language.
func TestHandshakeHeaders(t *testing.T) {
	seen := make(chan stdhttp.Header, 1)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		seen <- r.Header.Clone()
		// The fake server is not discord.com, so it must take discord.com's origin, which gorilla's upgrader refuses by default.
		upgrader := websocket.Upgrader{CheckOrigin: func(*stdhttp.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(server.Close)

	// The dialer's own TLS dial is for wss; a plain dialer shows what the handshake carries.
	dialer := websocket.Dialer{EnableCompression: true}
	conn, _, err := dialer.DialContext(t.Context(), "ws"+strings.TrimPrefix(server.URL, "http"), HandshakeHeaders())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	got := <-seen
	for _, name := range []string{"Pragma", "Cache-Control", "User-Agent", "Origin", "Accept-Encoding", "Accept-Language", "Sec-Websocket-Version", "Sec-Websocket-Extensions"} {
		if got.Get(name) == "" {
			t.Errorf("%s missing from %v", name, got)
		}
	}
	if got.Get("Origin") != "https://discord.com" || got.Get("Accept-Encoding") != "gzip, deflate, br, zstd" {
		t.Errorf("Origin %q, Accept-Encoding %q", got.Get("Origin"), got.Get("Accept-Encoding"))
	}
	for _, name := range []string{"Authorization", "X-Super-Properties", "Cookie"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s sent in the handshake", name)
		}
	}
}
