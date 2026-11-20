package httputil

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
)

type RequestOption func(httpdriver.Request) error
type ResponseFunc func(httpdriver.Request, httpdriver.Response) error

func PrependOptions(opts []RequestOption, prepend ...RequestOption) []RequestOption {
	if len(opts) == 0 {
		return prepend
	}
	return append(prepend, opts...)
}

func JSONRequest(r httpdriver.Request) error {
	r.AddHeader(http.Header{
		"Content-Type": {"application/json"},
	})
	return nil
}

func MultipartRequest(r httpdriver.Request) error {
	r.AddHeader(http.Header{
		"Content-Type": {"multipart/form-data"},
	})
	return nil
}

func WithHeaders(headers http.Header) RequestOption {
	return func(r httpdriver.Request) error {
		r.AddHeader(headers)
		return nil
	}
}

func WithContentType(ctype string) RequestOption {
	return func(r httpdriver.Request) error {
		r.AddHeader(http.Header{
			"Content-Type": {ctype},
		})
		return nil
	}
}

func WithSchema(schema SchemaEncoder, v any) RequestOption {
	return func(r httpdriver.Request) error {
		var params url.Values

		if p, ok := v.(url.Values); ok {
			params = p
		} else {
			p, err := schema.Encode(v)
			if err != nil {
				return err
			}
			params = p
		}

		r.AddQuery(params)
		return nil
	}
}

func WithBody(body io.ReadCloser) RequestOption {
	return func(r httpdriver.Request) error {
		r.WithBody(body)
		return nil
	}
}

// WithJSONBody inserts v as a JSON body, encoded whole so the request carries its Content-Length and, as a browser's does, no trailing newline.
func WithJSONBody(v any) RequestOption {
	if v == nil {
		return func(httpdriver.Request) error { return nil }
	}

	return func(r httpdriver.Request) error {
		body, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("encode JSON body: %w", err)
		}

		r.AddHeader(http.Header{
			"Content-Type": {"application/json"},
		})
		r.WithBody(httpdriver.Bytes(body))
		return nil
	}
}
