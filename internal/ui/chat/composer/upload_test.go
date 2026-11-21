package composer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/sendpart"
	"github.com/seamanticus/amitty/internal/ningen"
)

// puts records what was put where, failing with err.
type puts struct {
	got []string
	err error
}

func (p *puts) Put(_ context.Context, url string, body io.Reader, size int64) error {
	data, _ := io.ReadAll(body)
	if int64(len(data)) != size {
		return errors.New("size does not match the bytes")
	}
	p.got = append(p.got, url+" "+string(data))
	return p.err
}

func TestSendWithFiles(t *testing.T) {
	// evidence: witnesses
	// sent lists the requests the fake Discord took, as method, path and body.
	var sent []string
	client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent = append(sent, r.Method+" "+r.URL.Path+" "+string(body))
		if strings.HasSuffix(r.URL.Path, "/attachments") {
			_, _ = io.WriteString(w, `{"attachments":[{"id":0,"upload_url":"https://up.invalid/u/a.png","upload_filename":"u/a.png"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"5","channel_id":"1"}`)
	}))
	s := ningen.FromState(state.NewFromSession(session.NewCustom(gateway.DefaultIdentifier(""), client, handler.New()), defaultstore.New()))
	files := func() []sendpart.File {
		return []sendpart.File{{Name: "a.png", Reader: bytes.NewReader([]byte("hello"))}}
	}

	t.Run("each file is uploaded, then the message names it", func(t *testing.T) {
		sent = nil
		put := &puts{}
		if err := sendWithFiles(t.Context(), s, put, 1, api.SendMessageData{Content: "x", Nonce: "9"}, files()); err != nil {
			t.Fatal(err)
		}
		if len(put.got) != 1 || put.got[0] != "https://up.invalid/u/a.png hello" {
			t.Fatalf("put %v", put.got)
		}
		want := []string{
			`POST /api/v9/channels/1/attachments {"files":[{"filename":"a.png","file_size":5,"id":"0","is_clip":false,"original_content_type":"image/png"}]}`,
			`POST /api/v9/channels/1/messages {"mobile_network_type":"unknown","content":"x","nonce":"9","type":0,"tts":false,"attachments":[{"id":"0","filename":"a.png","uploaded_filename":"u/a.png","is_clip":false,"is_thumbnail":false}]}`,
		}
		if len(sent) != 2 || sent[0] != want[0] || sent[1] != want[1] {
			t.Fatalf("sent %q, want %q", sent, want)
		}
	})

	t.Run("a failed upload sends no message", func(t *testing.T) {
		sent = nil
		if err := sendWithFiles(t.Context(), s, &puts{err: errors.New("refused")}, 1, api.SendMessageData{}, files()); err == nil {
			t.Fatal("no error")
		}
		if len(sent) != 1 {
			t.Fatalf("sent %q", sent)
		}
	})
}
