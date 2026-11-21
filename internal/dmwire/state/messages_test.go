package state

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
)

// limitDriver answers every messages request with as many messages as it asks for, up to have, and records each limit asked.
type limitDriver struct {
	mu     sync.Mutex
	have   int
	limits []string
}

func (d *limitDriver) NewRequest(ctx context.Context, method, url string) (httpdriver.Request, error) {
	return httpdriver.NewMockRequestWithContext(ctx, method, url, nil, nil), nil
}

func (d *limitDriver) Do(request httpdriver.Request) (httpdriver.Response, error) {
	mock, ok := request.(*httpdriver.MockRequest)
	if !ok {
		return nil, errors.New("not a mock request")
	}
	limit := mock.URL.Query().Get("limit")
	d.mu.Lock()
	d.limits = append(d.limits, limit)
	d.mu.Unlock()
	n, err := strconv.Atoi(limit)
	if err != nil {
		return nil, err
	}
	messages := make([]discord.Message, min(n, d.have))
	for i := range messages {
		messages[i] = discord.Message{ID: discord.MessageID(1000 - i), ChannelID: 10}
	}
	return httpdriver.NewMockResponse(http.StatusOK, http.Header{}, messages), nil
}

func TestMessagesFetchesOnlyTheLimit(t *testing.T) {
	// evidence: witnesses
	for _, tt := range []struct {
		name       string
		limit      uint
		have       int
		wantLimits []string
		wantLen    int
	}{
		{"a busy channel is one request of the limit", 50, 500, []string{"50"}, 50},
		{"a small channel returns what it has", 50, 7, []string{"50"}, 7},
		{"no limit asks for a store's worth", 0, 500, []string{"100"}, 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			driver := &limitDriver{have: tt.have}
			client := api.NewCustomClient("token", httputil.NewClientWithDriver(driver))
			cabinet := defaultstore.New()
			if err := cabinet.ChannelSet(&discord.Channel{ID: 10, Type: discord.DirectMessage}, false); err != nil {
				t.Fatal(err)
			}
			state := NewFromSession(session.NewCustom(gateway.DefaultIdentifier("token"), client, handler.New()), cabinet)
			messages, err := state.Messages(10, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != tt.wantLen {
				t.Errorf("got %d messages, want %d", len(messages), tt.wantLen)
			}
			if len(driver.limits) != len(tt.wantLimits) || driver.limits[0] != tt.wantLimits[0] {
				t.Errorf("limits asked = %q, want %q", driver.limits, tt.wantLimits)
			}
		})
	}
}
