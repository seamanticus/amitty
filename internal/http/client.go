package http

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
	"github.com/seamanticus/amitty/internal/logger"
)

// NewClient returns an API client for token, or for none before login, that connects through network.
func NewClient(network Network, token string) *api.Client {
	httpClient := httputil.NewClientWithDriver(OmitEmptyHeaders(wrapClient(network.TLSClient(tls_client.WithCookieJar(tls_client.NewCookieJar())))))
	client := api.NewCustomClient(token, httpClient)
	client.UserAgent = BrowserUserAgent()
	return client
}

type client struct {
	tls_client.HttpClient
}

var _ httpdriver.Client = (*client)(nil)

func wrapClient(c tls_client.HttpClient) httpdriver.Client {
	return &client{HttpClient: c}
}

func (c *client) NewRequest(ctx context.Context, method, url string) (httpdriver.Request, error) {
	req, err := fhttp.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	orderHeaders(req.Header, apiHeaderOrder())
	return (*request)(req), nil
}

func (c *client) Do(req httpdriver.Request) (httpdriver.Response, error) {
	freq := (*fhttp.Request)(req.(*request))
	resp, err := c.HttpClient.Do(freq)
	if err != nil {
		return nil, err
	}
	if slog.Default().Enabled(freq.Context(), logger.LevelRaw) {
		resp.Body = &rawBody{ReadCloser: resp.Body, method: freq.Method, url: freq.URL.Redacted(), status: resp.StatusCode, ctx: freq.Context()}
	}
	return (*response)(resp), nil
}

// rawBody copies a response body as it is read, and logs it whole at LevelRaw when closed.
type rawBody struct {
	io.ReadCloser
	read        bytes.Buffer
	method, url string
	status      int
	ctx         context.Context
}

func (b *rawBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.read.Write(p[:n])
	return n, err
}

func (b *rawBody) Close() error {
	logger.Raw(b.ctx, "api response", b.read.Bytes(), "method", b.method, "url", b.url, "status", b.status)
	return b.ReadCloser.Close()
}

type request fhttp.Request

var _ httpdriver.Request = (*request)(nil)

func (r *request) GetPath() string {
	return r.URL.Path
}

func (r *request) GetContext() context.Context {
	return (*fhttp.Request)(r).Context()
}

func (r *request) AddHeader(header http.Header) {
	maps.Copy(r.Header, fhttp.Header(header))
}

func (r *request) AddQuery(values url.Values) {
	query := r.URL.Query()
	for key, value := range values {
		query[key] = append(query[key], value...)
	}
	r.URL.RawQuery = query.Encode()
}

func (r *request) WithBody(body io.ReadCloser) {
	r.Body = body
}

type response fhttp.Response

var _ httpdriver.Response = (*response)(nil)

func (r *response) GetStatus() int {
	return r.StatusCode
}

func (r *response) GetHeader() http.Header {
	return http.Header(r.Header)
}

func (r *response) GetBody() io.ReadCloser {
	return r.Body
}
