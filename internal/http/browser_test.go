package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserSharesState(t *testing.T) {
	var cookies, installations []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies = append(cookies, r.Header.Get("Cookie"))
		installations = append(installations, r.Header.Get("X-Installation-Id"))
		http.SetCookie(w, &http.Cookie{Name: "__dcfduid", Value: "d", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	browser := NewBrowser(Network{})
	if err := browser.Client("").FastRequest(http.MethodGet, server.URL+"/api/v9/experiments"); err != nil {
		t.Fatal(err)
	}
	browser.SetInstallation("i.x")
	browser.SetInstallation("")
	// Another client, as the login's next step or the chat makes, is the same browser.
	if err := browser.Client("t").FastRequest(http.MethodPost, server.URL+"/api/v9/auth/login"); err != nil {
		t.Fatal(err)
	}
	if cookies[0] != "" || installations[0] != "" {
		t.Errorf("first request sent cookie %q and installation %q, want neither", cookies[0], installations[0])
	}
	if cookies[1] != "__dcfduid=d" || installations[1] != "i.x" {
		t.Errorf("second request sent cookie %q and installation %q, want the first answer's cookie and i.x", cookies[1], installations[1])
	}
}
