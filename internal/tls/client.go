package tls

import (
	"fmt"
	"slices"

	tls_client "github.com/bogdanfinn/tls-client"
)

// NewClient returns a client with the Chrome profile's TLS and HTTP/2 fingerprint. HTTP/3 stays off: the profile has no QUIC settings, so its handshake would not be Chrome's, and an HTTP proxy cannot carry its UDP.
func NewClient(options ...tls_client.HttpClientOption) tls_client.HttpClient {
	options = slices.Concat([]tls_client.HttpClientOption{
		tls_client.WithClientProfile(clientProfile),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithDisableHttp3(),
	}, options)
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		panic(fmt.Errorf("failed to create new tls http client: %w", err))
	}
	return client
}
