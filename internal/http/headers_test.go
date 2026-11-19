package http

import (
	"bufio"
	"net"
	"net/textproto"
	"slices"
	"strings"
	"testing"
)

// rawHeaderNames serves one HTTP/1.1 request and returns its header names in the order they arrived.
func rawHeaderNames(t *testing.T, send func(url string)) []string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	names := make(chan []string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			names <- nil
			return
		}
		defer conn.Close() //nolint:errcheck
		reader := textproto.NewReader(bufio.NewReader(conn))
		var got []string
		if _, err := reader.ReadLine(); err == nil {
			for {
				line, err := reader.ReadLine()
				if err != nil || line == "" {
					break
				}
				name, _, _ := strings.Cut(line, ":")
				got = append(got, strings.ToLower(name))
			}
		}
		_, _ = conn.Write([]byte("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n"))
		names <- got
	}()
	send("http://" + listener.Addr().String() + "/api/v9/users/@me")
	return <-names
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
		got := rawHeaderNames(t, func(url string) {
			driver := OmitEmptyHeaders(wrapClient(Network{}.TLSClient()))
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
		if len(got) < 10 || !inOrder(got, want) {
			t.Fatalf("headers arrived as %v, want the order %v", got, want)
		}
		// The ordering key itself must never reach the wire.
		if slices.ContainsFunc(got, func(name string) bool { return strings.Contains(name, "order") }) {
			t.Fatalf("an ordering key was sent: %v", got)
		}
	})

	t.Run("media", func(t *testing.T) {
		got := rawHeaderNames(t, func(url string) {
			if resp, err := NewMedia(Network{}).Get(t.Context(), url); err == nil {
				_ = resp.Body.Close()
			}
		})
		if len(got) < 12 || !inOrder(got, imageHeaderOrder()) {
			t.Fatalf("headers arrived as %v, want the order %v", got, want)
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
			{"image", strings.Fields("sec-ch-ua-platform user-agent sec-ch-ua sec-ch-ua-mobile accept sec-fetch-site sec-fetch-mode sec-fetch-dest sec-fetch-storage-access referer accept-encoding accept-language cookie priority"), imageHeaderOrder()},
		} {
			if !inOrder(tt.got, tt.order) {
				t.Errorf("%s: the capture %v is out of the order %v", tt.name, tt.got, tt.order)
			}
		}
	})
}
