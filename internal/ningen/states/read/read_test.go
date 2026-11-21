package read

import (
	"context"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
)

type ackDriver struct {
	requests chan *httpdriver.MockRequest
}

func (d *ackDriver) NewRequest(ctx context.Context, method, url string) (httpdriver.Request, error) {
	return httpdriver.NewMockRequestWithContext(ctx, method, url, nil, nil), nil
}

func (d *ackDriver) Do(request httpdriver.Request) (httpdriver.Response, error) {
	d.requests <- request.(*httpdriver.MockRequest)
	return httpdriver.NewMockResponse(http.StatusOK, http.Header{}, struct{}{}), nil
}

func TestChannelUnreadUpdateSetsLastMessage(t *testing.T) {
	const (
		guildID   = discord.GuildID(1)
		channelID = discord.ChannelID(2)
		messageID = discord.MessageID(3)
	)

	cabinet := defaultstore.New()
	if err := cabinet.ChannelSet(&discord.Channel{ID: channelID, GuildID: guildID}, false); err != nil {
		t.Fatal(err)
	}

	h := handler.New()
	NewState(&state.State{Cabinet: cabinet}, h)

	event := &gateway.ChannelUnreadUpdateEvent{GuildID: guildID}
	event.ChannelUnreadUpdates = append(event.ChannelUnreadUpdates, struct {
		ID            discord.ChannelID `json:"id"`
		LastMessageID discord.MessageID `json:"last_message_id"`
	}{ID: channelID, LastMessageID: messageID})
	h.Call(event)

	channel, err := cabinet.Channel(channelID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.LastMessageID != messageID {
		t.Fatalf("LastMessageID = %d, want %d", channel.LastMessageID, messageID)
	}
}

func TestAckDay(t *testing.T) {
	// The NetLog's ack on 2026-11-20 UTC; the epoch is 2015-01-01.
	if got := api.AckDay(time.Date(2026, 11, 20, 19, 22, 0, 0, time.UTC)); got != 4341 {
		t.Fatalf("AckDay = %d, want 4341", got)
	}
}

func TestMarkReadAcksUncachedChannelCursor(t *testing.T) {
	const (
		channelID = discord.ChannelID(2)
		messageID = discord.MessageID(3)
	)

	driver := &ackDriver{requests: make(chan *httpdriver.MockRequest, 1)}
	client := api.NewCustomClient("token", httputil.NewClientWithDriver(driver))
	session := session.NewCustom(gateway.DefaultIdentifier("token"), client, handler.New())
	cabinet := defaultstore.New()
	if err := cabinet.ChannelSet(&discord.Channel{ID: channelID, GuildID: 1}, false); err != nil {
		t.Fatal(err)
	}

	stateHandler := handler.New()
	readHandler := handler.New()
	readState := NewState(&state.State{
		Session: session, Cabinet: cabinet, Handler: stateHandler,
	}, readHandler)
	readHandler.Call(&gateway.ReadyEvent{User: discord.User{ID: 4}})

	readState.MarkRead(channelID, messageID)

	select {
	case request := <-driver.requests:
		want := "/api/v9/channels/2/messages/3/ack"
		if request.GetPath() != want {
			t.Fatalf("ack path = %q, want %q", request.GetPath(), want)
		}
		// The web client's form, for a guild channel; only the day varies.
		if body := regexp.MustCompile(`^\{"token":null,"flags":1,"last_viewed":\d+\}$`); !body.Match(request.Body) {
			t.Fatalf("ack body = %s", request.Body)
		}
	case <-time.After(time.Second):
		t.Fatal("MarkRead did not acknowledge an uncached channel cursor")
	}
}

func TestMarkReadBulk(t *testing.T) {
	// evidence: witnesses
	driver := &ackDriver{requests: make(chan *httpdriver.MockRequest, 1)}
	client := api.NewCustomClient("token", httputil.NewClientWithDriver(driver))
	readHandler := handler.New()
	readState := NewState(&state.State{
		Session: session.NewCustom(gateway.DefaultIdentifier("token"), client, handler.New()), Cabinet: defaultstore.New(), Handler: handler.New(),
	}, readHandler)
	ready := &gateway.ReadyEvent{User: discord.User{ID: 4}}
	// Channel 5 is read already; 6 and 7 are not.
	ready.ReadStates = []gateway.ReadState{{ChannelID: 5, LastMessageID: 50}, {ChannelID: 6, LastMessageID: 60, MentionCount: 1}}
	readHandler.Call(ready)

	if err := readState.MarkReadBulk(map[discord.ChannelID]discord.MessageID{7: 71, 5: 50, 6: 61, 8: 0}); err != nil {
		t.Fatal(err)
	}
	request := <-driver.requests
	if want := `{"read_states":[{"channel_id":"6","message_id":"61","read_state_type":0},{"channel_id":"7","message_id":"71","read_state_type":0}]}`; request.GetPath() != "/api/v9/read-states/ack-bulk" || string(request.Body) != want {
		t.Fatalf("%s %s, want %s", request.GetPath(), request.Body, want)
	}
	if rs := readState.ReadState(6); rs.LastMessageID != 61 || rs.MentionCount != 0 {
		t.Fatalf("read state = %+v", rs)
	}

	if err := readState.MarkReadBulk(map[discord.ChannelID]discord.MessageID{6: 61}); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-driver.requests:
		t.Fatalf("acked what was read: %s", request.Body)
	default:
	}
}

func TestStateReadState(t *testing.T) {
	// read returns the state of user 4, who has read message 3 of forum 2, after events.
	read := func(t *testing.T, events ...ws.Event) *State {
		t.Helper()
		cabinet := defaultstore.New()
		if err := cabinet.ChannelSet(&discord.Channel{ID: 2, GuildID: 1, Type: discord.GuildForum}, false); err != nil {
			t.Fatal(err)
		}

		h := handler.New()
		s := NewState(&state.State{Cabinet: cabinet, Handler: handler.New()}, h)

		ready := &gateway.ReadyEvent{User: discord.User{ID: 4}}
		ready.ReadStates = []gateway.ReadState{{ChannelID: 2, LastMessageID: 3, MentionCount: 1}}
		for _, event := range append([]ws.Event{ready}, events...) {
			h.Call(event)
		}

		return s
	}

	created := func(ownerID discord.UserID, newlyCreated bool) *gateway.ThreadCreateEvent {
		return &gateway.ThreadCreateEvent{ID: 5, ParentID: 2, OwnerID: ownerID, NewlyCreated: newlyCreated}
	}

	t.Run("a deleted channel or thread leaves no read state", func(t *testing.T) {
		for _, event := range []ws.Event{
			&gateway.ChannelDeleteEvent{ID: 2},
			&gateway.ThreadDeleteEvent{ID: 2},
		} {
			if got := read(t, event).ReadState(2); got != nil {
				t.Fatalf("%T: read state = %+v, want none", event, got)
			}
		}
	})

	for name, tt := range map[string]struct {
		event *gateway.ThreadCreateEvent
		want  discord.MessageID
	}{
		"creating a thread in a forum reads the forum up to it":    {created(4, true), 5},
		"a thread someone else created leaves the forum as it was": {created(9, true), 3},
		"being added to a thread leaves the forum as it was":       {created(4, false), 3},
	} {
		t.Run(name, func(t *testing.T) {
			if got := read(t, tt.event).ReadState(2).LastMessageID; got != tt.want {
				t.Fatalf("LastMessageID = %d, want %d", got, tt.want)
			}
		})
	}
}
