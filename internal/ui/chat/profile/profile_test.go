package profile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

func TestModelView(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	state := ningen.FromState(discordtest.State(t))
	const guildID = 1
	if err := state.Cabinet.RoleSet(guildID, &discord.Role{ID: 3, Name: "Mods", Color: 0x00ff00, Position: 2}, false); err != nil {
		t.Fatal(err)
	}
	if err := state.Cabinet.RoleSet(guildID, &discord.Role{ID: 4, Name: "Fans", Position: 1}, false); err != nil {
		t.Fatal(err)
	}
	if err := state.Cabinet.PresenceSet(guildID, &discord.Presence{
		User:         discord.User{ID: 10},
		Status:       discord.IdleStatus,
		ClientStatus: discord.ClientStatus{Mobile: discord.IdleStatus},
		Activities:   []discord.Activity{{Type: discord.ListeningActivity, Name: "Spotify"}, {Type: discord.CustomActivity, State: "away"}},
	}, false); err != nil {
		t.Fatal(err)
	}
	user := discord.User{ID: 10, Username: "alice", PublicFlags: discord.EarlySupporter, PrimaryGuild: discord.PrimaryGuild{Enabled: true, Tag: "ABC"}}
	m, command := New(cfg, state, ShowMsg{GuildID: guildID, Member: discord.Member{User: user, Nick: "Alice", RoleIDs: []discord.RoleID{4, 3}}})
	if cfg.UI.MessagesList.Avatars.Preview && command == nil {
		t.Fatal("no avatar requested")
	}

	got := discordtest.Draw(t, m.View(), 60, 20)
	// evidence: witnesses
	for _, want := range []string{"Alice [ABC]", "alice", "Idle on mobile", "Listening to Spotify", "away", "Roles: Mods, Fans", "Badges: Early Supporter", "ID: 10"} {
		if !strings.Contains(got, want) {
			t.Errorf("view lacks %q:\n%s", want, got)
		}
	}

	t.Run("close", func(t *testing.T) {
		_, command := m.Update(tui.KeyMsg(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone)))
		task, ok := command.(cmd.Task)
		if !ok {
			t.Fatalf("command = %#v", command)
		}
		if _, ok := task().(ClosedMsg); !ok {
			t.Fatal("esc did not close the panel")
		}
	})
}

func TestBadgeNames(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		flags discord.UserFlags
		want  string
	}{
		{0, ""},
		{discord.Partner, "Partner"},
		{discord.Employee | discord.HouseBalance, "Staff, HypeSquad Balance"},
		{discord.TeamUser | discord.LikelySpammer, ""},
	} {
		if got := badgeNames(test.flags); got != test.want {
			t.Errorf("badgeNames(%d) = %q, want %q", test.flags, got, test.want)
		}
	}
}
