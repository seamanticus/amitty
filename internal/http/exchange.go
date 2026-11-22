package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httptrace"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/seamanticus/amitty/internal/logger"
)

// do sends req through client, logging it at debug level when sent. When the log takes LevelRaw, it also logs the request and its response as one record once the response body is closed, or the request fails; keepSent and keepReceived false log the size alone of the request and response body.
func do(client tls_client.HttpClient, req *fhttp.Request, keepSent, keepReceived bool) (*fhttp.Response, error) {
	ctx := req.Context()
	slog.DebugContext(ctx, "api request", "method", req.Method, "url", logURL(req.URL))
	if !slog.Default().Enabled(ctx, logger.LevelRaw) {
		return client.Do(req)
	}
	ex := &exchange{ctx: ctx, req: req, start: time.Now()}
	// The transport reports each header as written, its own included, so the log shows the wire's order.
	req = req.WithContext(httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{WroteHeaderField: ex.wrote}))
	ex.req = req
	if req.Body != nil && req.Body != fhttp.NoBody {
		ex.sent = &capture{ReadCloser: req.Body, keep: keepSent}
		req.Body = ex.sent
	}
	resp, err := client.Do(req)
	if err != nil {
		ex.log(nil, nil, err)
		return nil, err
	}
	resp.Body = &responseBody{ReadCloser: resp.Body, keep: keepReceived, ex: ex, resp: resp}
	return resp, nil
}

// logURL is u as the log shows it: userinfo and a token query parameter blanked.
func logURL(u *url.URL) string {
	query := u.Query()
	if !query.Has("token") {
		return u.Redacted()
	}
	query.Set("token", "<redacted>")
	copied := *u
	copied.RawQuery = query.Encode()
	return copied.Redacted()
}

// exchange is a request being logged.
type exchange struct {
	ctx   context.Context
	req   *fhttp.Request
	sent  *capture
	start time.Time

	mu    sync.Mutex
	wire  map[string][]string
	order []string
}

// wrote records a header the transport wrote.
func (ex *exchange) wrote(name string, values []string) {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	if ex.wire == nil {
		ex.wire = map[string][]string{}
	}
	// The first header again is the transport retrying the request: keep the last attempt.
	if len(ex.order) > 0 && ex.order[0] == name {
		ex.wire, ex.order = map[string][]string{}, nil
	}
	if _, ok := ex.wire[name]; !ok {
		ex.order = append(ex.order, name)
	}
	ex.wire[name] = append(ex.wire[name], values...)
}

// requestHeaders is what the transport wrote, in wire order, or the headers amitty set when it reported none.
func (ex *exchange) requestHeaders() slog.Value {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	if len(ex.order) == 0 {
		return logger.Headers(ex.req.Header, ex.req.Header[fhttp.HeaderOrderKey])
	}
	return logger.Headers(ex.wire, ex.order)
}

// log writes the exchange: the response and what was read of its body, or the error that left no response.
func (ex *exchange) log(resp *fhttp.Response, received *capture, err error) {
	attrs := []slog.Attr{
		slog.String("method", ex.req.Method),
		slog.String("url", logURL(ex.req.URL)),
	}
	if resp != nil {
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
	}
	attrs = append(attrs,
		slog.Int64("ms", time.Since(ex.start).Milliseconds()),
		slog.Group("request", slog.Any("headers", ex.requestHeaders()), slog.Any("body", ex.sent.body())),
	)
	if resp != nil {
		attrs = append(attrs, slog.Group("response", slog.Any("headers", logger.Headers(resp.Header, nil)), slog.Any("body", received.body())))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	slog.LogAttrs(ex.ctx, logger.LevelRaw, "api exchange", attrs...)
}

// capture counts what is read through a body, and keeps a copy when keep is set.
type capture struct {
	io.ReadCloser
	keep bool
	size int64
	data bytes.Buffer
}

func (c *capture) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.size += int64(n)
	if c.keep {
		c.data.Write(p[:n])
	}
	return n, err
}

// body is what the log shows of the body: its content, or its size when not kept.
func (c *capture) body() slog.Value {
	switch {
	case c == nil:
		return logger.Body(nil)
	case !c.keep && c.size > 0:
		return slog.StringValue(fmt.Sprintf("<%d bytes>", c.size))
	}
	return logger.Body(c.data.Bytes())
}

// responseBody logs its exchange when first closed, after reading what its caller left unread, as a caller that needs no body closes it unread.
type responseBody struct {
	capture
	ex     *exchange
	resp   *fhttp.Response
	logged bool
}

// maxUnread bounds how much of an unread body Close reads for the log.
const maxUnread = 1 << 20

func (b *responseBody) Close() error {
	if !b.logged {
		_, _ = io.Copy(io.Discard, io.LimitReader(&b.capture, maxUnread))
	}
	err := b.ReadCloser.Close()
	if !b.logged {
		b.logged = true
		b.ex.log(b.resp, &b.capture, nil)
	}
	return err
}
