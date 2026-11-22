package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/seamanticus/amitty/internal/logger"
)

// do sends req through client. When the log takes LevelRaw, it logs the request and its response as one record once the response body is closed, or the request fails; keepBody false logs the request body's size alone.
func do(client tls_client.HttpClient, req *fhttp.Request, keepBody bool) (*fhttp.Response, error) {
	ctx := req.Context()
	if !slog.Default().Enabled(ctx, logger.LevelRaw) {
		return client.Do(req)
	}
	slog.Debug("api request", "method", req.Method, "url", req.URL.Redacted())
	ex := &exchange{ctx: ctx, req: req, start: time.Now()}
	if req.Body != nil && req.Body != fhttp.NoBody {
		ex.sent = &capture{ReadCloser: req.Body, keep: keepBody}
		req.Body = ex.sent
	}
	resp, err := client.Do(req)
	if err != nil {
		ex.log(nil, nil, err)
		return nil, err
	}
	resp.Body = &responseBody{ReadCloser: resp.Body, keep: true, ex: ex, resp: resp}
	return resp, nil
}

// exchange is a request being logged.
type exchange struct {
	ctx   context.Context
	req   *fhttp.Request
	sent  *capture
	start time.Time
}

// log writes the exchange: the response and what was read of its body, or the error that left no response.
func (ex *exchange) log(resp *fhttp.Response, received *capture, err error) {
	attrs := []slog.Attr{
		slog.String("method", ex.req.Method),
		slog.String("url", ex.req.URL.Redacted()),
	}
	if resp != nil {
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
	}
	attrs = append(attrs,
		slog.Int64("ms", time.Since(ex.start).Milliseconds()),
		// The headers amitty set; the transport adds its own, such as content-length and host.
		slog.Group("request", slog.Any("headers", logger.Headers(ex.req.Header, ex.req.Header[fhttp.HeaderOrderKey])), slog.Any("body", ex.sent.body())),
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
