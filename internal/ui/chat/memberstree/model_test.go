package memberstree

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/ui/chat/profile"
)

const guildID, channelID = 1, 2

func newTestModel(t *testing.T) Model {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(cfg, ningen.FromState(discordtest.State(t)))
}

// shown returns the profile command shows, running only its last task, which follows the member list request.
func shown(command tui.Cmd) (profile.ShowMsg, bool) {
	batch, ok := command.(cmd.Batch)
	if !ok || len(batch) == 0 {
		return profile.ShowMsg{}, false
	}
	task, ok := batch[len(batch)-1].(cmd.Task)
	if !ok {
		return profile.ShowMsg{}, false
	}
	show, ok := task().(profile.ShowMsg)
	return show, ok
}

// setGuildMembers stores a guild channel whose member list holds items, given as JSON.
func setGuildMembers(t *testing.T, m Model, items string) *discord.Channel {
	t.Helper()
	channel := &discord.Channel{ID: channelID, GuildID: guildID, Type: discord.GuildText}
	if err := m.state.ChannelSet(channel, false); err != nil {
		t.Fatal(err)
	}
	if err := m.state.RoleSet(guildID, &discord.Role{ID: guildID, Permissions: discord.PermissionViewChannel}, false); err != nil {
		t.Fatal(err)
	}
	if err := m.state.RoleSet(guildID, &discord.Role{ID: 3, Name: "Mods"}, false); err != nil {
		t.Fatal(err)
	}

	event := &gateway.GuildMemberListUpdateEvent{ID: "everyone", GuildID: guildID}
	event.Ops = []gateway.GuildMemberListOp{{Op: "SYNC", Range: [2]int{0, 99}}}
	if err := json.Unmarshal([]byte(items), &event.Ops[0].Items); err != nil {
		t.Fatal(err)
	}
	m.state.State.Call(event)
	return channel
}

// lines returns the text of the groups and, indented, their members.
func lines(m Model) []string {
	var lines []string
	for _, group := range m.root.Children() {
		lines = append(lines, nodeText(group))
		for _, member := range group.Children() {
			lines = append(lines, "  "+nodeText(member))
		}
	}
	return lines
}

func nodeText(node *tree.Node) string {
	var s string
	for _, span := range node.Spans() {
		s += span.Text
	}
	return s
}

func TestModelShown(t *testing.T) {
	for _, tt := range []struct {
		name    string
		channel *discord.Channel
		want    bool
	}{
		{"none", nil, false},
		{"guild channel", &discord.Channel{GuildID: guildID, Type: discord.GuildText}, true},
		{"thread", &discord.Channel{GuildID: guildID, Type: discord.GuildPublicThread}, false},
		{"group dm", &discord.Channel{Type: discord.GroupDM}, true},
		{"dm", &discord.Channel{Type: discord.DirectMessage}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.channel = tt.channel
			if got := m.Shown(); got != tt.want {
				t.Fatalf("Shown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelRebuild(t *testing.T) {
	t.Run("guild members", func(t *testing.T) {
		m := newTestModel(t)
		m.channel = setGuildMembers(t, m, `[
			{"group": {"id": "3", "count": 1}},
			{"member": {"user": {"id": "10", "username": "a"}, "nick": "Alice"}},
			{"group": {"id": "online", "count": 2}},
			{"member": {"user": {"id": "11", "username": "b"}}},
			{},
			{"member": {"user": {"id": "12", "username": "c"}}}
		]`)
		m.rebuild()

		// The member after the unloaded item belongs to a group that has yet to arrive.
		want := []string{"Mods — 1", "  ● Alice", "Online — 2", "  ● b", "Members", "  ● c"}
		if got := lines(m); !slices.Equal(got, want) {
			t.Fatalf("lines = %q, want %q", got, want)
		}
	})

	t.Run("group dm recipients", func(t *testing.T) {
		m := newTestModel(t)
		if err := m.state.MyselfSet(discord.User{ID: 13, Username: "me"}, false); err != nil {
			t.Fatal(err)
		}
		if err := m.state.PresenceSet(discord.NoGuild(), &discord.Presence{User: discord.User{ID: 11}, Status: discord.OnlineStatus}, false); err != nil {
			t.Fatal(err)
		}
		m.channel = &discord.Channel{Type: discord.GroupDM, DMRecipients: []discord.User{
			{ID: 10, Username: "a"},
			{ID: 11, Username: "c"},
			{ID: 12, Username: "B"},
		}}
		m.rebuild()

		// Online members come first, then members by name, including the current user.
		want := []string{"Members — 4", "  ● c", "  ● a", "  ● B", "  ● me"}
		if got := lines(m); !slices.Equal(got, want) {
			t.Fatalf("lines = %q, want %q", got, want)
		}
	})

	t.Run("keeps selection and collapsed groups", func(t *testing.T) {
		m := newTestModel(t)
		m.channel = setGuildMembers(t, m, `[
			{"group": {"id": "online", "count": 1}},
			{"member": {"user": {"id": "10", "username": "a"}}},
			{"group": {"id": "offline", "count": 1}},
			{"member": {"user": {"id": "11", "username": "b"}}}
		]`)
		m.rebuild()
		online, offline := m.root.Children()[0], m.root.Children()[1]
		selected := online.Children()[0]
		m.current = selected
		offline.Collapse()

		m.rebuild()
		if got := m.current; got != selected || m.root.PathTo(got) == nil {
			t.Fatal("the selected member was not kept in the tree")
		}
		if m.root.Children()[1].Expanded() {
			t.Fatal("the collapsed group was expanded")
		}
	})
}

func TestModelProfile(t *testing.T) {
	m := newTestModel(t)
	m.channel = setGuildMembers(t, m, `[
		{"group": {"id": "3", "count": 1}},
		{"member": {"user": {"id": "10", "username": "a"}, "nick": "Alice", "roles": ["3"]}}
	]`)
	m.rebuild()
	group := m.root.Children()[0]
	// evidence: witnesses
	t.Run("a member shows their profile", func(t *testing.T) {
		_, command := m.Update(selectedMsg{node: group.Children()[0]})
		show, ok := shown(command)
		if !ok || show.GuildID != guildID || show.Member.Nick != "Alice" || show.Member.User.ID != 10 {
			t.Fatalf("shown = %+v, %v", show, ok)
		}
	})
	t.Run("a group shows none", func(t *testing.T) {
		if _, command := m.Update(selectedMsg{node: group}); command != nil {
			if _, ok := shown(command); ok {
				t.Fatal("a group showed a profile")
			}
		}
	})
	t.Run("a DM recipient", func(t *testing.T) {
		m.channel = &discord.Channel{Type: discord.DirectMessage, DMRecipients: []discord.User{{ID: 20, Username: "z"}}}
		if show, ok := m.profile(20); !ok || show.GuildID.IsValid() || show.Member.User.Username != "z" {
			t.Fatalf("profile = %+v, %v", show, ok)
		}
		if _, ok := m.profile(21); ok {
			t.Fatal("a stranger has a profile")
		}
	})
}
