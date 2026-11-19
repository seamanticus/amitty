package session

import (
	"net/http"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
)

func TestAuthTokenRefresh(t *testing.T) {
	s := NewCustom(gateway.DefaultIdentifier("old"), api.NewCustomClient("old", httputil.NewClientWithDriver(httpdriver.WrapClient(http.Client{}))), handler.New())
	s.Handler.Call(&gateway.ReadyEvent{AuthToken: "new"})
	if s.Client.Session.Token != "new" {
		t.Fatalf("unexpected token: %q", s.Client.Session.Token)
	}
}

func TestOpenWithoutGateway(t *testing.T) {
	s := NewCustom(gateway.DefaultIdentifier("t"), api.NewCustomClient("t", httputil.NewClientWithDriver(httpdriver.WrapClient(http.Client{}))), handler.New())
	if err := s.Open(t.Context()); err == nil {
		t.Fatal("a session without a gateway opened")
	}
}
