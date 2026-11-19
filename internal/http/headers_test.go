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
	want := headerOrder()

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
		if len(got) < 3 || !inOrder(got, want) {
			t.Fatalf("headers arrived as %v, want the order %v", got, want)
		}
	})

	t.Run("every listed name is lowercase and listed once", func(t *testing.T) {
		for i, name := range want {
			if name != strings.ToLower(name) || slices.Index(want, name) != i {
				t.Fatalf("%q", name)
			}
		}
	})
}
