package chat

import (
	"io"
	"net/http"
	"testing"

	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

func TestCreateDM(t *testing.T) {
	var sent int
	client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// The request the web client sends, byte for byte.
		if r.Method != http.MethodPost || r.URL.Path != "/api/v9/users/@me/channels" || string(body) != `{"recipients":["3"]}` || r.Header.Get("X-Context-Properties") != "e30=" {
			http.Error(w, r.Method+" "+r.URL.Path+" "+string(body), http.StatusBadRequest)
			return
		}
		sent++
		_, _ = io.WriteString(w, `{"id":"9","type":1,"recipients":[{"id":"3","username":"c"}]}`)
	}))
	s := ningen.FromState(state.NewFromSession(session.NewCustom(gateway.DefaultIdentifier(""), client, handler.New()), defaultstore.New()))
	task, ok := createDM(s, 3).(cmd.Task)
	if !ok {
		t.Fatal("not a task")
	}
	if msg := task(); msg != (dmStartedMsg{9}) || sent != 1 {
		t.Fatalf("msg = %#v, %d sent", msg, sent)
	}
	if _, err := s.Cabinet.Channel(9); err != nil {
		t.Fatal("the new DM is not stored")
	}
}
