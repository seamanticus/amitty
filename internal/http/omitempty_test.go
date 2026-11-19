package http_test

import (
	"net/http"
	"testing"

	"github.com/seamanticus/discordo/internal/discordtest"
)

// evidence: witnesses
func TestOmitEmptyHeaders(t *testing.T) {
	var header http.Header
	client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		_, _ = w.Write([]byte(`{}`))
	}))
	// A client without a token, as before login: arikawa still sets Authorization to "".
	if _, err := client.Me(); err != nil {
		t.Fatal(err)
	}
	if values, sent := header["Authorization"]; sent {
		t.Fatalf("Authorization sent as %q, want absent", values)
	}
	if header.Get("User-Agent") == "" {
		t.Fatal("non-empty headers were dropped too")
	}

	client.Token = "t"
	if _, err := client.Me(); err != nil {
		t.Fatal(err)
	}
	if got := header.Get("Authorization"); got != "t" {
		t.Fatalf("Authorization = %q, want the token", got)
	}
}
