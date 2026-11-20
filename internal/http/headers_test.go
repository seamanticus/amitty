package http

import (
	"crypto/tls"
	"io"
	"net"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bogdanfinn/fhttp/http2"
	"github.com/bogdanfinn/fhttp/http2/hpack"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// rawHeaderNames serves one HTTP/2 request over TLS, as discord.com gets them, and returns its header names in the order its HEADERS frame carried them. HTTP/1.1 would not do: fhttp appends the Accept-Encoding it adds there after the ordered headers.
func rawHeaderNames(t *testing.T, send func(client tls_client.HttpClient, url string)) []string {
	t.Helper()
	// httptest makes the certificate; the frames are read by hand, as a server's Header map has no order.
	cert := httptest.NewUnstartedServer(nil)
	cert.EnableHTTP2 = true
	cert.StartTLS()
	config := cert.TLS.Clone()
	cert.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	names := make(chan []string, 1)
	go func() {
		names <- readHeaderNames(listener)
	}()
	send(Network{}.TLSClient(tls_client.WithInsecureSkipVerify()), "https://"+listener.Addr().String()+"/api/v9/users/@me")
	return <-names
}

// readHeaderNames accepts one HTTP/2 connection and returns the regular header names of its first request, or nil.
func readHeaderNames(listener net.Listener) []string {
	conn, err := listener.Accept()
	if err != nil {
		return nil
	}
	defer conn.Close() //nolint:errcheck
	tlsConn, ok := conn.(*tls.Conn)
	if !ok || tlsConn.Handshake() != nil || tlsConn.ConnectionState().NegotiatedProtocol != "h2" {
		return nil
	}
	preface := make([]byte, len(http2.ClientPreface))
	if _, err := io.ReadFull(conn, preface); err != nil || string(preface) != http2.ClientPreface {
		return nil
	}
	framer := http2.NewFramer(conn, conn)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if framer.WriteSettings() != nil {
		return nil
	}
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return nil
		}
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if !frame.IsAck() && framer.WriteSettingsAck() != nil {
				return nil
			}
		case *http2.MetaHeadersFrame:
			var got []string
			for _, field := range frame.RegularFields() {
				got = append(got, field.Name)
			}
			return got
		}
	}
}

// inOrder reports whether every name of want that got has comes in want's order.
func inOrder(got, want []string) bool {
	last := -1
	for _, name := range got {
		i := slices.Index(want, name)
		if i < 0 {
			continue
		}
		if i < last {
			return false
		}
		last = i
	}
	return true
}

func TestHeaderOrder(t *testing.T) {
	want := apiHeaderOrder()

	t.Run("API requests", func(t *testing.T) {
		got := rawHeaderNames(t, func(client tls_client.HttpClient, url string) {
			driver := OmitEmptyHeaders(wrapClient(client))
			req, err := driver.NewRequest(t.Context(), "GET", url)
			if err != nil {
				t.Fatal(err)
			}
			headers := Headers(Build{})
			headers.Set("Authorization", "token")
			headers.Set("User-Agent", BrowserUserAgent())
			req.AddHeader(headers)
			if resp, err := driver.Do(req); err == nil {
				_ = resp.GetBody().Close()
			}
		})
		// accept-encoding is fhttp's own; it must still land in its place.
		if len(got) < 10 || !inOrder(got, want) || !slices.Contains(got, "accept-encoding") {
			t.Fatalf("headers arrived as %v, want the order %v", got, want)
		}
		// The ordering key itself must never reach the wire.
		if slices.ContainsFunc(got, func(name string) bool { return strings.Contains(name, "order") }) {
			t.Fatalf("an ordering key was sent: %v", got)
		}
	})

	t.Run("a JSON body is sent with its length", func(t *testing.T) {
		got := rawHeaderNames(t, func(client tls_client.HttpClient, url string) {
			driver := OmitEmptyHeaders(wrapClient(client))
			req, err := driver.NewRequest(t.Context(), "POST", url)
			if err != nil {
				t.Fatal(err)
			}
			if err := httputil.WithJSONBody(map[string][]string{"recipients": {"1"}})(req); err != nil {
				t.Fatal(err)
			}
			if resp, err := driver.Do(req); err == nil {
				_ = resp.GetBody().Close()
			}
		})
		if len(got) == 0 || got[0] != "content-length" {
			t.Fatalf("headers arrived as %v, want content-length first", got)
		}
	})

	t.Run("media", func(t *testing.T) {
		got := rawHeaderNames(t, func(client tls_client.HttpClient, url string) {
			if resp, err := (Media{client: client}).Get(t.Context(), url); err == nil {
				_ = resp.Body.Close()
			}
		})
		if len(got) < 12 || !inOrder(got, imageHeaderOrder()) {
			t.Fatalf("headers arrived as %v, want the order %v", got, imageHeaderOrder())
		}
	})

	t.Run("every listed name is lowercase and listed once", func(t *testing.T) {
		for _, order := range [][]string{want, imageHeaderOrder()} {
			for i, name := range order {
				if name != strings.ToLower(name) || slices.Index(order, name) != i {
					t.Fatalf("%q", name)
				}
			}
		}
	})

	// The orders the NetLog of Brave 157 showed, header names only, sec-gpc left out.
	t.Run("captured orders", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			got   []string
			order []string
		}{
			{"API GET", strings.Fields("sec-ch-ua-platform authorization x-installation-id x-debug-options sec-ch-ua sec-ch-ua-mobile x-discord-timezone x-super-properties x-discord-locale user-agent accept sec-fetch-site sec-fetch-mode sec-fetch-dest referer accept-encoding accept-language cookie priority"), want},
			{"API GET with context", strings.Fields("sec-ch-ua-platform authorization x-installation-id x-debug-options sec-ch-ua sec-ch-ua-mobile x-discord-timezone x-context-properties x-super-properties x-discord-locale user-agent accept sec-fetch-site sec-fetch-mode sec-fetch-dest referer accept-encoding accept-language cookie priority"), want},
			{"API POST", strings.Fields("content-length sec-ch-ua-platform authorization x-installation-id x-debug-options sec-ch-ua sec-ch-ua-mobile x-discord-timezone x-super-properties x-discord-locale user-agent content-type accept origin sec-fetch-site sec-fetch-mode sec-fetch-dest referer accept-encoding accept-language cookie priority"), want},
			{"API PUT with context", strings.Fields("content-length sec-ch-ua-platform authorization x-installation-id x-debug-options sec-ch-ua sec-ch-ua-mobile x-discord-timezone x-context-properties x-super-properties x-discord-locale user-agent content-type accept origin sec-fetch-site sec-fetch-mode sec-fetch-dest referer accept-encoding accept-language cookie priority"), want},
			{"API PUT captcha retry", strings.Fields("content-length x-captcha-rqtoken sec-ch-ua-platform authorization x-installation-id x-debug-options sec-ch-ua sec-ch-ua-mobile x-discord-timezone x-captcha-key x-context-properties x-super-properties x-captcha-session-id x-discord-locale user-agent content-type accept origin sec-fetch-site sec-fetch-mode sec-fetch-dest referer accept-encoding accept-language cookie priority"), want},
			{"image", strings.Fields("sec-ch-ua-platform user-agent sec-ch-ua sec-ch-ua-mobile accept sec-fetch-site sec-fetch-mode sec-fetch-dest sec-fetch-storage-access referer accept-encoding accept-language cookie priority"), imageHeaderOrder()},
		} {
			if !inOrder(tt.got, tt.order) {
				t.Errorf("%s: the capture %v is out of the order %v", tt.name, tt.got, tt.order)
			}
		}
	})
}
