package http

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// evidence: witnesses
func TestParseNetwork(t *testing.T) {
	for _, tt := range []struct {
		proxy, want string
		ok          bool
	}{
		{"", "direct", true},
		{"http://127.0.0.1:3128", "http://127.0.0.1:3128", true},
		{"https://proxy.example:443", "https://proxy.example:443", true},
		{"socks5://127.0.0.1:1080", "socks5://127.0.0.1:1080", true},
		{"socks5h://127.0.0.1:9050", "socks5h://127.0.0.1:9050", true},
		{"socks5h://user:secret@127.0.0.1:9050", "socks5h://user:xxxxx@127.0.0.1:9050", true},
		{"http://[::1]:3128", "http://[::1]:3128", true},
		{"socks4://127.0.0.1:1080", "", false},
		{"socks4a://127.0.0.1:1080", "", false},
		{"ftp://127.0.0.1", "", false},
		{"127.0.0.1:9050", "", false},
		{"//127.0.0.1:9050", "", false},
		{"http://", "", false},
		{"http://127.0.0.1:3128/path", "", false},
		{"http://127.0.0.1:3128?q=1", "", false},
		{"http://127.0.0.1:3128#f", "", false},
		{"http://bad host:1", "", false},
		{"%", "", false},
	} {
		t.Run(tt.proxy, func(t *testing.T) {
			network, err := ParseNetwork(tt.proxy)
			if (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok %v", err, tt.ok)
			}
			if err != nil {
				if strings.Contains(err.Error(), "secret") {
					t.Fatalf("error shows the password: %v", err)
				}
				return
			}
			if got := network.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("a rejected URL keeps its password out of the error", func(t *testing.T) {
		if _, err := ParseNetwork("socks4://user:secret@127.0.0.1:1080"); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("err = %v", err)
		}
	})
}

// tunnelProxy is an HTTP CONNECT proxy, as tls-client speaks to every http and https proxy, that relays to target whatever host is asked for, and counts tunnels.
func tunnelProxy(t *testing.T, target string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var tunnels atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		tunnels.Add(1)
		upstream, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer upstream.Close() //nolint:errcheck
		w.WriteHeader(http.StatusOK)
		client, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer client.Close() //nolint:errcheck
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = io.Copy(upstream, rw)
		}()
		_, _ = io.Copy(client, upstream)
		<-done
	}))
	t.Cleanup(proxy.Close)
	return proxy, &tunnels
}

func TestMediaUsesTheProxy(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "image")
	}))
	t.Cleanup(media.Close)
	proxy, tunnels := tunnelProxy(t, media.Listener.Addr().String())
	network, err := ParseNetwork(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	// The name does not resolve: only the proxy can reach the server.
	resp, err := NewMedia(network).Get(t.Context(), "http://media.invalid/a.png")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "image" || tunnels.Load() != 1 {
		t.Fatalf("body %q, %d tunnels", body, tunnels.Load())
	}
}

func TestDownProxyFailsClosed(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	direct := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request went around the proxy")
	}))
	direct.Listener = listener
	direct.Start()
	t.Cleanup(direct.Close)

	// A port nothing listens on, taken and released.
	gone, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := gone.Addr().String()
	_ = gone.Close()
	network, err := ParseNetwork("http://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	if resp, err := NewMedia(network).Get(t.Context(), direct.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("got a response with the proxy down")
	}
}

func TestZeroMedia(t *testing.T) {
	if _, err := (Media{}).Get(context.Background(), "http://media.invalid/"); err == nil {
		t.Fatal("the zero Media fetched")
	}
}
