package gateway

import (
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/gorilla/websocket"
	"github.com/seamanticus/amitty/internal/http"
)

// NewDialer returns a websocket dialer with the browser's TLS fingerprint that connects through network.
func NewDialer(network http.Network) websocket.Dialer {
	client := network.TLSClient(tls_client.WithForceHttp1())
	dialer := *websocket.DefaultDialer
	// tls-client's TLS dialer goes through network's proxy itself; gorilla's proxy would wrap it and lose the fingerprint.
	dialer.Proxy = nil
	dialer.EnableCompression = true
	dialer.NetDialTLSContext = client.GetTLSDialer()
	return dialer
}
