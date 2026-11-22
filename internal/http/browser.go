package http

import (
	"net/http"
	"sync"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
)

// Browser is the state the web client keeps between requests, which Discord ties to a device: its cookies and its installation ID. Every API client of a run shares one, as one browser tab would.
type Browser struct {
	network Network
	jar     tls_client.CookieJar

	mu           sync.Mutex
	installation string
}

// NewBrowser returns a browser with no cookies and no installation, connecting through network.
func NewBrowser(network Network) *Browser {
	return &Browser{network: network, jar: tls_client.NewCookieJar()}
}

// Network is how the browser connects.
func (b *Browser) Network() Network {
	return b.network
}

// Installation returns the installation ID, or "" before Discord gave one.
func (b *Browser) Installation() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.installation
}

// SetInstallation keeps id, which Discord gives in the experiments response and in READY when the one sent is missing or invalid. An empty id changes nothing.
func (b *Browser) SetInstallation(id string) {
	if id == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.installation = id
}

// Client returns an API client for token, or for none before login, that sends the browser's cookies and installation ID and keeps the cookies Discord sets.
func (b *Browser) Client(token string) *api.Client {
	httpClient := httputil.NewClientWithDriver(OmitEmptyHeaders(wrapClient(b.network.TLSClient(tls_client.WithCookieJar(b.jar)))))
	client := api.NewCustomClient(token, httpClient)
	client.UserAgent = BrowserUserAgent()
	client.OnRequest = append(client.OnRequest, b.installationHeader)
	return client
}

func (b *Browser) installationHeader(r httpdriver.Request) error {
	if id := b.Installation(); id != "" {
		r.AddHeader(http.Header{"X-Installation-Id": {id}})
	}
	return nil
}
