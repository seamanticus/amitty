package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
)

func TestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // lol

	client := NewCustomClient("no. 3-chan", httputil.NewClientWithDriver(httpdriver.WrapClient(http.Client{}))).WithContext(ctx)

	// This should fail.
	_, err := client.Me()
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatal("Unexpected error:", err)
	}
}
