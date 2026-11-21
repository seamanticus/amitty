package guildstree

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/ningen/states/read"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/ui"
)

func TestModelMoveDMToFront(t *testing.T) {
	t.Run("moves rendered channel to front", func(t *testing.T) {
		firstID := discord.ChannelID(1)
		secondID := discord.ChannelID(2)
		thirdID := discord.ChannelID(3)

		first := tree.NewNode("first").SetReference(firstID)
		second := tree.NewNode("second").SetReference(secondID)
		third := tree.NewNode("third").SetReference(thirdID)
		dmRoot := tree.NewNode("Direct Messages").SetChildren([]*tree.Node{first, second, third})
		m := Model{
			dmRootNode: dmRoot,
			channelNodes: map[discord.ChannelID]*tree.Node{
				firstID:  first,
				secondID: second,
				thirdID:  third,
			},
		}

		m.moveDMToFront(thirdID)

		want := []*tree.Node{third, first, second}
		if !slices.Equal(dmRoot.Children(), want) {
			t.Fatalf("children = %v, want %v", dmRoot.Children(), want)
		}
	})

	t.Run("ignores unrendered channel", func(t *testing.T) {
		firstID := discord.ChannelID(1)
		first := tree.NewNode("first").SetReference(firstID)
		dmRoot := tree.NewNode("Direct Messages").SetChildren([]*tree.Node{first})
		m := Model{
			dmRootNode:   dmRoot,
			channelNodes: map[discord.ChannelID]*tree.Node{},
		}

		m.moveDMToFront(discord.ChannelID(2))

		children := dmRoot.Children()
		if len(children) != 1 || children[0] != first {
			t.Fatalf("children = %v, want the original DM node", children)
		}
	})
}

func TestAdjacentNode(t *testing.T) {
	a, b, c := tree.NewNode("a"), tree.NewNode("b"), tree.NewNode("c")
	root := tree.NewNode("").SetChildren([]*tree.Node{a, tree.NewNode("guild").AddChild(b), c})
	match := func(n *tree.Node) bool { return n == a || n == b }
	for _, test := range []struct {
		name          string
		current       *tree.Node
		after, before *tree.Node
	}{
		{"none", nil, a, b},
		{"a", a, b, b},
		{"b", b, a, a},
		{"c", c, a, b},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Run("after", func(t *testing.T) {
				if got := adjacentNode(root, test.current, match, false); got != test.after {
					t.Errorf("got %v, want %v", got, test.after)
				}
			})
			t.Run("before", func(t *testing.T) {
				if got := adjacentNode(root, test.current, match, true); got != test.before {
					t.Errorf("got %v, want %v", got, test.before)
				}
			})
		})
	}
	t.Run("no match", func(t *testing.T) {
		if got := adjacentNode(root, a, func(*tree.Node) bool { return false }, false); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestMentionsLabel(t *testing.T) {
	for _, test := range []struct {
		name  string
		count int
		want  string
	}{
		{"none", 0, ""},
		{"some", 5, " (5)"},
		{"most counted", maxMentions, " (99)"},
		{"more", maxMentions + 1, " (99+)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := mentionsLabel(test.count); got != test.want {
				t.Errorf("mentionsLabel(%d) = %q, want %q", test.count, got, test.want)
			}
		})
	}
}

func TestModelMentions(t *testing.T) {
	s := ningen.FromState(discordtest.State(t))
	s.State.Call(&gateway.ReadyEvent{ReadStates: []gateway.ReadState{
		{ChannelID: 1, LastMessageID: 10, MentionCount: 2},
		{ChannelID: 2, LastMessageID: 20, MentionCount: 3},
	}})
	m := Model{state: s}
	channels := []discord.Channel{{ID: 1}, {ID: 2}, {ID: 3}}
	if got := m.mentions(channels...); got != 5 {
		t.Errorf("mentions = %d, want 5", got)
	}
}

func TestModelRefreshReadState(t *testing.T) {
	t.Run("keeps the name of a channel sharing its guild's ID", func(t *testing.T) {
		s := ningen.FromState(discordtest.State(t))
		me := discord.User{ID: 2}
		guild := discord.Guild{ID: 1, Name: "guild", OwnerID: me.ID}
		channel := discord.Channel{ID: 1, GuildID: guild.ID, Type: discord.GuildText, Name: "general"}
		if err := errors.Join(
			s.MyselfSet(me, false),
			s.GuildSet(&guild, false),
			s.MemberSet(guild.ID, &discord.Member{User: me}, false),
			s.RoleSet(guild.ID, &discord.Role{ID: discord.RoleID(guild.ID)}, false),
			s.ChannelSet(&channel, false),
		); err != nil {
			t.Fatal(err)
		}
		cfg := config.Default()
		m := NewModel(&cfg, s)
		m.createGuildNode(m.root, guild)
		guildNode := m.root.Children()[0]
		m.createChannelNode(guildNode, channel)
		channelNode := guildNode.Children()[0]

		m.refreshReadState(&read.UpdateEvent{ChannelID: 2, GuildID: guild.ID})

		if got, want := guildNode.Spans()[0].Text, m.guildText(guild); got != want {
			t.Errorf("guild node text = %q, want %q", got, want)
		}
		if got, want := channelNode.Spans()[0].Text, m.channelText(channel); got != want {
			t.Errorf("channel node text = %q, want %q", got, want)
		}
	})
}

func TestModelJoinedGuild(t *testing.T) {
	// evidence: witnesses
	cfg := config.Default()
	m := NewModel(&cfg, ningen.FromState(discordtest.State(t)))
	m.reset()
	m.createGuildNode(m.root, discord.Guild{ID: 1, Name: "old"})

	joined := &state.GuildJoinEvent{GuildCreateEvent: &gateway.GuildCreateEvent{ID: 2, Name: "new"}}
	m, _ = m.Update(joined)
	m, _ = m.Update(joined)
	var refs []any
	for _, node := range m.root.Children() {
		refs = append(refs, node.Reference())
	}
	if want := []any{dmNode{}, discord.GuildID(2), discord.GuildID(1)}; !slices.Equal(refs, want) {
		t.Fatalf("children = %v, want %v", refs, want)
	}

	if m, _ = m.Update(ShowGuildMsg{Guild: discord.Guild{ID: 2}}); m.current != m.guildNodes[2] {
		t.Fatalf("current = %v", m.current)
	}
	if m, _ = m.Update(ShowGuildMsg{Guild: discord.Guild{ID: 3, Name: "early"}}); m.current != m.guildNodes[3] || m.root.Children()[1] != m.guildNodes[3] {
		t.Fatal("a guild the gateway has not listed yet is not shown first")
	}
}

func TestModelMarkRead(t *testing.T) {
	// evidence: witnesses
	s := ningen.FromState(discordtest.State(t))
	me := discord.User{ID: 9}
	guild := discord.Guild{ID: 1, Name: "g", OwnerID: me.ID}
	channels := []discord.Channel{
		{ID: 11, GuildID: 1, Type: discord.GuildText, LastMessageID: 110},
		{ID: 12, GuildID: 1, Type: discord.GuildText, LastMessageID: 120},
		{ID: 13, GuildID: 1, Type: discord.GuildText, LastMessageID: 130},
	}
	if err := errors.Join(
		s.MyselfSet(me, false),
		s.GuildSet(&guild, false),
		s.MemberSet(guild.ID, &discord.Member{User: me}, false),
		s.RoleSet(guild.ID, &discord.Role{ID: discord.RoleID(guild.ID)}, false),
	); err != nil {
		t.Fatal(err)
	}
	for _, channel := range channels {
		if err := s.ChannelSet(&channel, false); err != nil {
			t.Fatal(err)
		}
	}
	// 11 and 12 are unread, 13 is read.
	s.State.Call(&gateway.ReadyEvent{User: me, ReadStates: []gateway.ReadState{
		{ChannelID: 11, LastMessageID: 100}, {ChannelID: 12, LastMessageID: 100}, {ChannelID: 13, LastMessageID: 130},
	}})
	cfg := config.Default()
	m := NewModel(&cfg, s)
	m.reset()
	folder := tree.NewNode("f")
	m.root.AddChild(folder)
	m.createGuildNode(folder, guild)

	if got := m.unreadIn([]discord.GuildID{1}); len(got) != 2 || got[11] != 110 || got[12] != 120 {
		t.Fatalf("unread = %v", got)
	}
	for _, node := range []*tree.Node{m.guildNodes[1], folder} {
		task, ok := m.confirmMarkRead(node).(cmd.Task)
		if !ok {
			t.Fatal("nothing asked")
		}
		dialog, _ := task().(ui.DialogMsg)
		if !strings.HasPrefix(dialog.Text, "Mark 2 channels in ") || len(dialog.Buttons) != 2 || dialog.Buttons[0].Label != "No" || dialog.Buttons[0].Cmd != nil {
			t.Fatalf("dialog = %+v", dialog)
		}
	}
	if m.confirmMarkRead(m.dmRootNode) != nil || m.confirmMarkRead(nil) != nil || m.confirmMarkRead(m.root) != nil {
		t.Fatal("asked to mark the DMs, the root or nothing read")
	}
}

func TestModelMessageRequests(t *testing.T) {
	// accepted and rejected count the requests the fake Discord answered.
	var accepted, rejected int
	client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v9/channels/7/recipients/@me" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			if body, _ := io.ReadAll(r.Body); strings.TrimSpace(string(body)) != `{"consent_status":2}` {
				http.Error(w, string(body), http.StatusBadRequest)
				return
			}
			accepted++
			_, _ = io.WriteString(w, `{"id":"7","type":1,"recipients":[{"id":"3","username":"c"}]}`)
		case http.MethodDelete:
			rejected++
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	s := ningen.FromState(state.NewFromSession(session.NewCustom(gateway.DefaultIdentifier(""), client, handler.New()), defaultstore.New()))
	request := discord.Channel{ID: 7, Type: discord.DirectMessage, DMRecipients: []discord.User{{ID: 3, Username: "c"}}, IsMessageRequest: true}
	friend := discord.Channel{ID: 8, Type: discord.DirectMessage, DMRecipients: []discord.User{{ID: 4, Username: "d"}}}
	for _, channel := range []discord.Channel{request, friend} {
		if err := s.ChannelSet(&channel, false); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	m := NewModel(&cfg, s)
	key := func(r string) tui.KeyMsg { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }
	// confirm presses a key on the node of id and runs the dialog's Yes, returning the dialog and the message Yes returns.
	confirm := func(id discord.ChannelID, r string) (*ui.DialogMsg, tui.Msg) {
		m.current = tree.NewNode("").SetReference(id)
		_, command := m.Update(key(r))
		task, ok := command.(cmd.Task)
		if !ok {
			return nil, nil
		}
		dialog, ok := task().(ui.DialogMsg)
		if !ok || len(dialog.Buttons) != 2 || dialog.Buttons[0].Label != "No" {
			t.Fatalf("dialog = %#v, want No first", dialog)
		}
		return &dialog, dialog.Buttons[1].Cmd.(cmd.Task)()
	}

	// evidence: witnesses
	if !strings.HasSuffix(m.channelText(request), requestLabel) || strings.Contains(m.channelText(friend), requestLabel) {
		t.Error("the request is not marked, or a DM is")
	}
	if dialog, _ := confirm(8, "a"); dialog != nil {
		t.Error("a DM that is no request asked to be accepted")
	}
	if _, msg := confirm(7, "a"); msg != (requestsChangedMsg{}) || accepted != 1 {
		t.Fatalf("accept = %#v, %d sent", msg, accepted)
	}
	if channel, err := s.Cabinet.Channel(7); err != nil || channel.IsMessageRequest {
		t.Fatal("the accepted DM is still a request")
	}
	if err := s.ChannelSet(&request, true); err != nil {
		t.Fatal(err)
	}
	if _, msg := confirm(7, "r"); msg != (requestsChangedMsg{}) || rejected != 1 {
		t.Fatalf("reject = %#v, %d sent", msg, rejected)
	}
	if _, err := s.Cabinet.Channel(7); err == nil {
		t.Fatal("the rejected DM is still stored")
	}
}
