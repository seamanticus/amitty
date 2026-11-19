package http

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/seamanticus/amitty/internal/tls"
)

// proxySchemes are the proxies a Network takes. SOCKS4 is left out: it has no authentication and plain socks4 resolves names locally.
var proxySchemes = []string{"http", "https", "socks5", "socks5h"}

// Network is how every connection reaches Discord: directly, or through one proxy that nothing bypasses. Its zero value is a direct connection.
type Network struct {
	// proxy is a checked proxy URL, or "" for a direct connection.
	proxy string
}

// ParseNetwork returns the network through proxy, a URL such as "socks5h://127.0.0.1:9050", or a direct one for "". SOCKS5 resolves names at the proxy for both socks5 and socks5h.
func ParseNetwork(proxy string) (Network, error) {
	if proxy == "" {
		return Network{}, nil
	}
	u, err := url.Parse(proxy)
	if err != nil {
		return Network{}, fmt.Errorf("proxy: %w", err)
	}
	switch {
	case !slices.Contains(proxySchemes, u.Scheme):
		return Network{}, fmt.Errorf("proxy %s: scheme %q is not one of %v", u.Redacted(), u.Scheme, proxySchemes)
	case u.Hostname() == "":
		return Network{}, fmt.Errorf("proxy %s: no host", u.Redacted())
	case u.Path != "" || u.RawQuery != "" || u.Fragment != "":
		return Network{}, fmt.Errorf("proxy %s: a proxy URL has no path, query or fragment", u.Redacted())
	}
	// tls-client checks the URL again when a client is made; a failure there would be a panic, so it surfaces here.
	if _, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), tls_client.WithProxyUrl(proxy)); err != nil {
		return Network{}, fmt.Errorf("proxy %s: %w", u.Redacted(), err)
	}
	return Network{proxy: proxy}, nil
}

// String names the network for logs, without the proxy's password.
func (n Network) String() string {
	if n.proxy == "" {
		return "direct"
	}
	u, err := url.Parse(n.proxy)
	if err != nil {
		panic("invariant: a parsed proxy URL no longer parses: " + err.Error())
	}
	return u.Redacted()
}

// TLSClient returns a client with the browser's TLS fingerprint that connects through n.
func (n Network) TLSClient(options ...tls_client.HttpClientOption) tls_client.HttpClient {
	if n.proxy != "" {
		options = append(options, tls_client.WithProxyUrl(n.proxy))
	}
	return tls.NewClient(options...)
}

// Media fetches attachments and images from Discord's CDN, through the same network and fingerprint as the API.
type Media struct {
	client tls_client.HttpClient
}

// NewMedia returns a Media that connects through network.
func NewMedia(network Network) Media {
	return Media{client: network.TLSClient()}
}

// Get requests url, an attachment or an image.
func (m Media) Get(ctx context.Context, url string) (*fhttp.Response, error) {
	if m.client == nil {
		return nil, errors.New("media client not set up")
	}
	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", BrowserUserAgent())
	return m.client.Do(req)
}
