package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

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
}
