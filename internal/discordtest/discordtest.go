// Package discordtest helps tests of the login: API clients served locally, so tests never reach Discord, and drawn widgets.
package discordtest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ayn2op/arikawa/v3/api"
	"github.com/ayn2op/arikawa/v3/utils/httputil"
	"github.com/ayn2op/arikawa/v3/utils/httputil/httpdriver"
	"github.com/ayn2op/tview"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	dhttp "github.com/seamanticus/discordo/internal/http"
)

// Client returns an API client without a token whose requests handler serves, with the path and query Discord would get.
func Client(t testing.TB, handler http.Handler) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := redirect{target: target, base: server.Client().Transport}
	driver := dhttp.OmitEmptyHeaders(httpdriver.WrapClient(http.Client{Transport: transport}))
	return api.NewCustomClient("", httputil.NewClientWithDriver(driver))
}

// ClosedClient returns an API client whose server has already shut down, so every request fails to connect.
func ClosedClient(t testing.TB) *api.Client {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := redirect{target: target, base: server.Client().Transport}
	server.Close()
	return api.NewCustomClient("", httputil.NewClientWithDriver(dhttp.OmitEmptyHeaders(httpdriver.WrapClient(http.Client{Transport: transport}))))
}

// Draw draws widget on a width x height mock terminal and returns the text drawn, one line per row.
func Draw(t testing.TB, widget tview.Widget, width, height int) string {
	t.Helper()
	screen, err := tcell.NewTerminfoScreenFromTty(vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)}))
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)

	widget.Draw(screen, tview.Rectangle{Width: width, Height: height})
	var drawn strings.Builder
	for y := range height {
		for x := range width {
			str, _, _ := screen.Get(x, y)
			drawn.WriteString(str)
		}
		drawn.WriteByte('\n')
	}
	return drawn.String()
}

// redirect sends every request to target instead of its own host.
type redirect struct {
	target *url.URL
	base   http.RoundTripper
}

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host, req.Host = r.target.Scheme, r.target.Host, r.target.Host
	return r.base.RoundTrip(req)
}
