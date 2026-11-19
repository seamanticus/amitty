package gateway

import (
	"net/http"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	dhttp "github.com/seamanticus/amitty/internal/http"
)

// HandshakeHeaders returns the headers Chromium adds to the web client's websocket handshake, as a NetLog of Brave 157 shows; the dialer adds Host, Connection, Upgrade and the Sec-WebSocket-* ones. Brave's language and its cookies are left out.
func HandshakeHeaders() http.Header {
	return http.Header{
		"Pragma":          {"no-cache"},
		"Cache-Control":   {"no-cache"},
		"User-Agent":      {dhttp.BrowserUserAgent()},
		"Origin":          {api.BaseEndpoint},
		"Accept-Encoding": {"gzip, deflate, br, zstd"},
		"Accept-Language": {"en-US,en;q=0.9"},
	}
}
