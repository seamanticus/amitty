package chat

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/chat/joinprompt"
)

func TestParseInvite(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		input, code string
		ok          bool
	}{
		{"YbN4TsWA8", "YbN4TsWA8", true},
		{"  YbN4TsWA8\n", "YbN4TsWA8", true},
		{"https://discord.gg/YbN4TsWA8", "YbN4TsWA8", true},
		{"discord.gg/YbN4TsWA8/", "YbN4TsWA8", true},
		{"https://discord.com/invite/YbN4TsWA8?event=1", "YbN4TsWA8", true},
		{"https://canary.discord.com/invite/a-b", "a-b", true},
		{"https://discordapp.com/invite/abc", "abc", true},
		{"", "", false},
		{"https://discord.com/channels/1/2", "", false},
		{"https://example.com/YbN4TsWA8", "", false},
		{"two words", "", false},
		{"../x", "", false},
	} {
		if code, ok := parseInvite(test.input); code != test.code || ok != test.ok {
			t.Errorf("parseInvite(%q) = %q, %v, want %q, %v", test.input, code, ok, test.code, test.ok)
		}
	}
}

// joinState is a state whose requests reach handler, connected as gateway session "s".
func joinState(t *testing.T, handle http.HandlerFunc) *ningen.State {
	t.Helper()
	s := state.NewFromSession(session.NewCustom(gateway.DefaultIdentifier(""), discordtest.Client(t, handle), handler.New()), defaultstore.New())
	// The session's handler, which the state stores events from; the state's own runs after.
	s.Session.Call(&gateway.ReadyEvent{SessionID: "s"})
	return ningen.FromState(s)
}

// task runs command, which must be a task, and returns its message.
func task(t *testing.T, command tui.Cmd) tui.Msg {
	t.Helper()
	run, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command = %#v, want a task", command)
	}
	return run()
}

func TestJoinGuild(t *testing.T) {
	// evidence: witnesses
	invite := discord.Invite{Code: "c", Guild: &discord.Guild{ID: 3, Name: "g"}, Channel: discord.Channel{ID: 4}}

	t.Run("the join is sent once, as the session, and answers the guild", func(t *testing.T) {
		var sent int
		s := joinState(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || r.URL.Path != "/api/v9/invites/c" || string(body) != `{"session_id":"s"}` {
				http.Error(w, r.Method+" "+r.URL.Path+" "+string(body), http.StatusBadRequest)
				return
			}
			sent++
			_, _ = io.WriteString(w, `{"code":"c","new_member":true,"guild":{"id":"3","name":"g","features":["GUILD_ONBOARDING"]}}`)
		})
		msg, ok := task(t, joinGuild(s, invite, captcha.Solution{})).(guildJoinedMsg)
		if !ok || msg.guild.ID != 3 || sent != 1 {
			t.Fatalf("msg = %#v, %d sent", msg, sent)
		}
	})

	t.Run("a challenge asks for a captcha for the invite", func(t *testing.T) {
		s := joinState(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"captcha_key":["You need to update your app to join this server."],"captcha_sitekey":"sk","captcha_service":"hcaptcha"}`)
		})
		if _, ok := task(t, joinGuild(s, invite, captcha.Solution{})).(ui.CaptchaMsg[inviteCode]); !ok {
			t.Fatal("no captcha")
		}
	})

	t.Run("an unknown invite says so", func(t *testing.T) {
		s := joinState(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message": "Unknown Invite", "code": 10006}`)
		})
		msg, ok := task(t, lookUpInvite(s, "https://discord.gg/zzzzzzzzz")).(joinFailedMsg)
		if !ok || !strings.Contains(msg.status, "not found") {
			t.Fatalf("msg = %#v", msg)
		}
	})

	t.Run("text that is no invite sends nothing", func(t *testing.T) {
		s := joinState(t, func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("sent %s %s", r.Method, r.URL)
		})
		if _, ok := task(t, lookUpInvite(s, "https://example.com/x")).(joinFailedMsg); !ok {
			t.Fatal("not refused")
		}
	})
}

func TestModelJoin(t *testing.T) {
	// evidence: witnesses
	invite := discord.Invite{Code: "c", Guild: &discord.Guild{ID: 3, Name: "g"}, ApproximateMembers: 12}

	t.Run("an invite to a new server asks first, No first", func(t *testing.T) {
		m := newTestModel(t)
		m.openJoin()
		dialog, ok := task(t, m.confirmJoin(invite)).(ui.DialogMsg)
		if !ok || dialog.Text != "Join g (12 members)?" || len(dialog.Buttons) != 2 || dialog.Buttons[0].Label != "No" || dialog.Buttons[0].Cmd != nil {
			t.Fatalf("dialog = %#v", dialog)
		}
		if yes, ok := task(t, dialog.Buttons[1].Cmd).(joinGuildMsg); !ok || yes.invite.Code != "c" {
			t.Fatalf("Yes = %#v", yes)
		}
	})

	t.Run("an invite to a server you are in opens it and sends nothing", func(t *testing.T) {
		m := newTestModel(t)
		if err := m.state.Cabinet.GuildSet(invite.Guild, false); err != nil {
			t.Fatal(err)
		}
		m.openJoin()
		m.confirmJoin(invite)
		if m.overlay != overlayNone || m.focused != paneGuildsTree || len(m.outreach) != 0 {
			t.Fatalf("overlay = %v, focused = %v, outreach = %v", m.overlay, m.focused, m.outreach)
		}
	})

	t.Run("joins share the outreach limit and count once each", func(t *testing.T) {
		m := newTestModel(t)
		m.openJoin()
		now := time.Now()
		for range outreachLimit {
			m.startJoin(invite, now)
		}
		if len(m.outreach) != outreachLimit {
			t.Fatalf("counted %d", len(m.outreach))
		}
		if dialog, ok := task(t, m.startJoin(invite, now)).(ui.DialogMsg); !ok || !strings.Contains(dialog.Text, "Try again later") {
			t.Fatalf("dialog = %#v", dialog)
		}
	})

	t.Run("an answered captcha sends the join again without counting", func(t *testing.T) {
		m := newTestModel(t)
		m.openJoin()
		m.startJoin(invite, time.Now())
		if _, command := m.Update(joinprompt.SubmittedMsg{Input: "c"}); command != nil {
			t.Fatal("a submit during a join looked up another")
		}
		if command := m.updateJoin(ui.CaptchaDoneMsg[inviteCode]{Key: "c"}); command == nil || len(m.outreach) != 1 {
			t.Fatalf("command = %v, outreach = %v", command, m.outreach)
		}
		if command := m.updateJoin(ui.CaptchaDoneMsg[inviteCode]{Key: "other"}); command != nil {
			t.Fatal("the captcha of another join sent this one")
		}
	})

	t.Run("a server that gates new members says where to finish", func(t *testing.T) {
		m := newTestModel(t)
		for _, test := range []struct {
			features []discord.GuildFeature
			told     bool
		}{
			{nil, false},
			{[]discord.GuildFeature{discord.News}, false},
			{[]discord.GuildFeature{discord.MemberVerificationGateEnabled}, true},
			{[]discord.GuildFeature{discord.GuildOnboarding}, true},
		} {
			told := false
			for _, command := range m.showJoined(discord.Guild{ID: 3, Name: "g", Features: test.features}).(cmd.Batch) {
				if run, ok := command.(cmd.Task); ok {
					if dialog, ok := run().(ui.DialogMsg); ok && strings.Contains(dialog.Text, "web client or the mobile app") {
						told = true
					}
				}
			}
			if told != test.told {
				t.Errorf("features %v: told = %v", test.features, told)
			}
		}
	})

	t.Run("esc closes the prompt", func(t *testing.T) {
		m := newTestModel(t)
		m.openJoin()
		next, _ := m.Update(task(t, m.route(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone))))
		if next.overlay != overlayNone {
			t.Fatalf("overlay = %v", next.overlay)
		}
	})
}
