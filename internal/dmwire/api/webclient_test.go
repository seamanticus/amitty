package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
)

// sent is a request a fake Discord received.
type sent struct {
	method, uri, body, context string
}

// fakeClient returns a client whose requests reach a server that records them and answers answer.
func fakeClient(t *testing.T, answer string) (*Client, *[]sent) {
	t.Helper()
	var got []sent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, sent{r.Method, r.URL.RequestURI(), string(body), r.Header.Get("X-Context-Properties")})
		if answer == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := rewrite{target, server.Client().Transport}
	return NewCustomClient("", httputil.NewClientWithDriver(httpdriver.WrapClient(http.Client{Transport: transport}))), &got
}

// rewrite sends every request to target, keeping its path.
type rewrite struct {
	target *url.URL
	base   http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host, req.Host = r.target.Scheme, r.target.Host, r.target.Host
	return r.base.RoundTrip(req)
}

// The requests below are those a NetLog of the web client showed, byte for byte.
func TestWebClientRequests(t *testing.T) {
	t.Run("send", func(t *testing.T) {
		client, got := fakeClient(t, `{"id":"5"}`)
		if _, err := client.SendMessageComplex(1, SendMessageData{Content: "hello"}); err != nil {
			t.Fatal(err)
		}
		body := regexp.MustCompile(`^\{"mobile_network_type":"unknown","content":"hello","nonce":"\d+","tts":false,"flags":0\}$`)
		if len(*got) != 1 || !body.MatchString((*got)[0].body) || (*got)[0].context != chatInputContext {
			t.Fatalf("sent %+v", *got)
		}
	})
	t.Run("react and unreact", func(t *testing.T) {
		client, got := fakeClient(t, "")
		emoji := discord.APIEmoji("👍")
		if err := client.React(1, 2, emoji); err != nil {
			t.Fatal(err)
		}
		if err := client.Unreact(1, 2, emoji); err != nil {
			t.Fatal(err)
		}
		want := []sent{
			{"PUT", "/api/v9/channels/1/messages/2/reactions/%F0%9F%91%8D/%40me?location=Message%20Hover%20Bar&type=0", "", ""},
			{"DELETE", "/api/v9/channels/1/messages/2/reactions/%F0%9F%91%8D/0/%40me?location=Message%20Hover%20Bar&burst=false", "", ""},
		}
		if len(*got) != len(want) || (*got)[0] != want[0] || (*got)[1] != want[1] {
			t.Fatalf("sent %+v, want %+v", *got, want)
		}
	})
	t.Run("join a server", func(t *testing.T) {
		client, got := fakeClient(t, `{"code":"YbN4TsWA8","guild":{"id":"3"},"channel":{"id":"4","type":0}}`)
		invite, err := client.InviteToJoin("YbN4TsWA8", "https://discord.gg/YbN4TsWA8")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.JoinInvite(*invite, "0123456789abcdef0123456789abcdef"); err != nil {
			t.Fatal(err)
		}
		want := []sent{
			{"GET", "/api/v9/invites/YbN4TsWA8?inputValue=https%3A%2F%2Fdiscord.gg%2FYbN4TsWA8&with_counts=true&with_expiration=true&with_permissions=true", "", ""},
			// {"location":"Join Guild","location_guild_id":"3","location_channel_id":"4","location_channel_type":0}
			{"POST", "/api/v9/invites/YbN4TsWA8", `{"session_id":"0123456789abcdef0123456789abcdef"}`, "eyJsb2NhdGlvbiI6IkpvaW4gR3VpbGQiLCJsb2NhdGlvbl9ndWlsZF9pZCI6IjMiLCJsb2NhdGlvbl9jaGFubmVsX2lkIjoiNCIsImxvY2F0aW9uX2NoYW5uZWxfdHlwZSI6MH0="},
		}
		if len(*got) != len(want) || (*got)[0] != want[0] || (*got)[1] != want[1] {
			t.Fatalf("sent %+v, want %+v", *got, want)
		}
	})
}
