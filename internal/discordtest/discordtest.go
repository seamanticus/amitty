// Package discordtest helps tests of the login: API clients served locally, so tests never reach Discord, and drawn widgets.
package discordtest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
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

// State returns an empty state of no account, with an empty store, whose API client can only reach a closed server, so a test never reaches Discord.
func State(t testing.TB) *state.State {
	t.Helper()
	return state.NewFromSession(session.NewCustom(gateway.DefaultIdentifier(""), ClosedClient(t), handler.New()), defaultstore.New())
}

// Draw draws widget on a width x height mock terminal and returns the text drawn, one line per row.
func Draw(t testing.TB, widget tui.Widget, width, height int) string {
	t.Helper()
	screen, err := tcell.NewTerminfoScreenFromTty(vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)}))
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)

	widget.Draw(screen, tui.Rectangle{Width: width, Height: height})
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
