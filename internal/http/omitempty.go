package http

import (
	"context"
	"net/http"

	"github.com/ayn2op/arikawa/v3/utils/httputil/httpdriver"
)

// OmitEmptyHeaders returns driver with headers whose only value is empty left out, as a browser never sends them. arikawa sets Authorization to the token even when there is none, which would otherwise send an empty Authorization before login.
func OmitEmptyHeaders(driver httpdriver.Client) httpdriver.Client {
	return omitEmpty{driver}
}

type omitEmpty struct{ driver httpdriver.Client }

type omitEmptyRequest struct{ httpdriver.Request }

func (c omitEmpty) NewRequest(ctx context.Context, method, url string) (httpdriver.Request, error) {
	r, err := c.driver.NewRequest(ctx, method, url)
	if err != nil {
		return nil, err
	}
	return omitEmptyRequest{r}, nil
}

func (c omitEmpty) Do(req httpdriver.Request) (httpdriver.Response, error) {
	if r, ok := req.(omitEmptyRequest); ok {
		req = r.Request
	}
	return c.driver.Do(req)
}

func (r omitEmptyRequest) AddHeader(header http.Header) {
	kept := make(http.Header, len(header))
	for key, values := range header {
		if len(values) == 1 && values[0] == "" {
			continue
		}
		kept[key] = values
	}
	r.Request.AddHeader(kept)
}
