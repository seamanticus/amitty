package messageslist

import (
	"encoding/json"
	"image"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/markdown"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	tuiimage "github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/tint"
	"github.com/seamanticus/amitty/internal/ui/chat/profile"
	"github.com/seamanticus/amitty/internal/ui/emojis"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// newTestModel returns a list showing channel 1, logged in as user 1.
func newTestModel(t *testing.T, cfg *config.Config) *Model {
	t.Helper()
	ml := NewModel(cfg, ningen.FromState(discordtest.State(t)), dhttp.Media{})
	if err := ml.state.MyselfSet(discord.User{ID: 1}, false); err != nil {
		t.Fatal(err)
	}
	ml.channel = &discord.Channel{ID: 1}
	return &ml
}

// evidence: witnesses
func TestModelViewAuthorProfile(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	if ml.viewAuthorProfile() != nil {
		t.Fatal("a profile without a selected message")
	}
	ml.setMessages([]discord.Message{{ID: 2, Author: discord.User{ID: 7, Username: "a"}}})
	ml.selectBottom()
	task, ok := ml.viewAuthorProfile().(cmd.Task)
	if !ok {
		t.Fatal("no profile for the selected message")
	}
	show, ok := task().(profile.ShowMsg)
	if !ok || show.Member.User.ID != 7 || show.GuildID.IsValid() {
		t.Fatalf("got %#v, want author 7 of a DM", show)
	}

	// In a server, the author's member is shown when the state has it.
	ml.channel = &discord.Channel{ID: 1, GuildID: 5}
	if err := ml.state.MemberSet(5, &discord.Member{User: discord.User{ID: 7}, Nick: "n"}, false); err != nil {
		t.Fatal(err)
	}
	task, _ = ml.viewAuthorProfile().(cmd.Task)
	if show, _ := task().(profile.ShowMsg); show.GuildID != 5 || show.Member.Nick != "n" {
		t.Fatalf("got %#v, want the member of guild 5", show)
	}
}

func TestModelBuildItem(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	ml.setMessages([]discord.Message{{ID: 3}, {ID: 2}})
	// kept reports whether the newest message keeps the view it was shown with.
	kept := func() bool { return ml.items[ml.messageIndex(len(ml.items), -1)].view != nil }

	t.Run("oldest first", func(t *testing.T) {
		if ml.items[ml.messageIndex(-1, 1)].message.ID != 2 {
			t.Fatal("messages are not oldest first")
		}
	})
	t.Run("keeps the view", func(t *testing.T) {
		ml.buildItem(ml.messageIndex(len(ml.items), -1))
		if !kept() {
			t.Fatal("message view was not kept")
		}
	})
	t.Run("fetches older messages once at the top", func(t *testing.T) {
		t.Run("select top", func(t *testing.T) {
			if ml.selectTop() == nil {
				t.Fatal("older messages were not fetched on reaching the top")
			}
		})
		t.Run("select top again", func(t *testing.T) {
			if ml.selectTop() != nil {
				t.Fatal("older messages were fetched again")
			}
		})
		t.Run("select up at the top", func(t *testing.T) {
			if ml.selectUp() != nil {
				t.Fatal("older messages were fetched again")
			}
		})
	})
	t.Run("fetches older messages on clicking the top", func(t *testing.T) {
		ml.selectBottom()
		if _, cmd := ml.Update(listMsg(0)); cmd == nil {
			t.Fatal("clicking the top message did not fetch older messages")
		}
	})
	t.Run("prepend", func(t *testing.T) {
		ml.selectBottom()
		*ml, _ = ml.Update(olderMessagesLoadedMsg{channelID: 1, older: []discord.Message{{ID: 1}}})
		if selected, ok := ml.selectedMessage(); !ok || selected.ID != 3 || !kept() {
			t.Fatal("prepend lost selection or cached view")
		}
	})
	t.Run("a page already shown is not prepended again", func(t *testing.T) {
		count := len(ml.items)
		*ml, _ = ml.Update(olderMessagesLoadedMsg{channelID: 1, older: []discord.Message{{ID: 1}}})
		if len(ml.items) != count {
			t.Fatalf("items %d, want %d", len(ml.items), count)
		}
	})
	t.Run("delete", func(t *testing.T) {
		ml.deleteMessage(ml.messageIndex(-1, 1))
		if !kept() {
			t.Fatal("deletion discarded another message's view")
		}
	})
	t.Run("edit", func(t *testing.T) {
		ml.setMessage(ml.messageIndex(len(ml.items), -1), discord.Message{ID: 3, Content: "edited"})
		if kept() {
			t.Fatal("edit reused stale view")
		}
	})
	t.Run("invalidate", func(t *testing.T) {
		ml.buildItem(ml.messageIndex(len(ml.items), -1))
		ml.InvalidateRendered()
		if kept() {
			t.Fatal("global invalidation reused stale view")
		}
	})
	t.Run("reset", func(t *testing.T) {
		ml.reset()
		if len(ml.items) != 0 {
			t.Fatal("reset retained messages or rows")
		}
	})

	t.Run("layout", func(t *testing.T) {
		cfg := testConfig(t)
		cfg.UI.MessagesList.Compact = true
		ml := newTestModel(t, cfg)
		limits := layout.Limits{Max: layout.Size{Width: 80}, Infinite: layout.Axes{Height: true}}

		// The avatar is 2 rows, below the row of a reply.
		avatar := tuiimage.New(image.NewAlpha(image.Rect(0, 0, 4, 4))).Width(4).CellSize(1, 2)
		ml.images = map[discord.URL]tuiimage.Widget{avatarURL(discord.User{}): avatar}
		for _, test := range []struct {
			name        string
			messageType discord.MessageType
			want        int
		}{
			{"default", discord.DefaultMessage, 2},
			{"reply", discord.InlinedReplyMessage, 3},
		} {
			t.Run(test.name, func(t *testing.T) {
				ml.SetChannel(ml.channel, []discord.Message{{ID: 1, Type: test.messageType, Content: "hi"}})
				if got := ml.buildItem(ml.messageIndex(-1, 1)).Layout(limits).Height; got != test.want {
					t.Fatalf("height = %d, want %d", got, test.want)
				}
			})
		}
	})
}

func TestModelRebuildItems(t *testing.T) {
	for _, tt := range []struct {
		name       string
		separators bool
	}{
		{name: "separators_disabled", separators: false},
		{name: "separators_enabled", separators: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.DateSeparator.Enabled = tt.separators
			ml := newTestModel(t, cfg)
			var messages []discord.Message
			if err := json.Unmarshal([]byte(`[
   {"id":"3","timestamp":"2026-09-03T12:00:00Z"},
   {"id":"2","timestamp":"2026-09-02T12:00:00Z"},
   {"id":"1","timestamp":"2026-09-01T12:00:00Z"}
  ]`), &messages); err != nil {
				t.Fatal(err)
			}
			ml.setMessages(messages[:2])
			assertItems := func(t *testing.T, want ...discord.MessageID) {
				t.Helper()
				var got []discord.MessageID
				for i, item := range ml.items {
					if item.separator != nil {
						if !tt.separators || i+1 == len(ml.items) || ml.items[i+1].separator != nil {
							t.Fatalf("item %d: orphaned separator", i)
						}
					} else {
						got = append(got, item.message.ID)
					}
				}
				if !slices.Equal(got, want) {
					t.Fatalf("messages = %v, want %v", got, want)
				}
				expected := len(want)
				if tt.separators {
					expected *= 2
				}
				if len(ml.items) != expected {
					t.Fatalf("item count = %d, want %d", len(ml.items), expected)
				}
			}
			assertSelected := func(t *testing.T, want discord.MessageID) {
				t.Helper()
				message, ok := ml.selectedMessage()
				if !ok || message.ID != want {
					t.Fatalf("selected = %v, want %v", message, want)
				}
			}
			t.Run("set", func(t *testing.T) {
				assertItems(t, 2, 3)
			})
			t.Run("select top", func(t *testing.T) {
				ml.selectTop()
				assertSelected(t, 2)
			})
			t.Run("select down", func(t *testing.T) {
				ml.selectDown()
				assertSelected(t, 3)
			})
			t.Run("select up", func(t *testing.T) {
				ml.selectUp()
				assertSelected(t, 2)
			})
			t.Run("prepend older", func(t *testing.T) {
				*ml, _ = ml.Update(olderMessagesLoadedMsg{channelID: 1, older: messages[2:]})
				assertItems(t, 1, 2, 3)
				assertSelected(t, 2)
			})
			t.Run("delete the middle", func(t *testing.T) {
				ml.deleteMessage(ml.cursor)
				assertItems(t, 1, 3)
				assertSelected(t, 1)
			})
			t.Run("delete the first", func(t *testing.T) {
				ml.deleteMessage(ml.cursor)
				assertItems(t, 3)
				assertSelected(t, 3)
			})
			t.Run("delete the last", func(t *testing.T) {
				ml.deleteMessage(ml.cursor)
				assertItems(t)
				if ml.cursor != -1 {
					t.Fatal("empty list retained selection")
				}
			})
		})
	}
}

func TestModelMessageURLs(t *testing.T) {
	t.Run("schemes", func(t *testing.T) {
		ml := newTestModel(t, testConfig(t))
		message := discord.Message{Content: "[a](file:///etc/passwd) [b](ms-msdt:id) [c](HTTPS://example.com) https://example.org"}

		if got, want := ml.messageURLs(message, nil), []string{"HTTPS://example.com", "https://example.org"}; !slices.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("embeds disabled", func(t *testing.T) {
		ml := newTestModel(t, testConfig(t))
		ml.cfg.UI.MessagesList.Embeds.Enabled = false
		message := discord.Message{Embeds: []discord.Embed{{URL: "https://example.org"}}}

		if got := ml.messageURLs(message, nil); len(got) != 0 {
			t.Errorf("got %q, want none while embeds are disabled", got)
		}
	})
}

func TestModelDrawLayout(t *testing.T) {
	cfg := testConfig(t)
	// A layout without a reference time is drawn as it is, whatever the time zone.
	cfg.Timestamps.Format = "T"
	ml := newTestModel(t, cfg)
	message := discord.Message{Author: discord.User{Username: "a"}, Content: "hi", EditedTimestamp: discord.NowTimestamp()}

	for _, test := range []struct {
		name    string
		compact bool
		want    string
	}{
		{"cozy", false, "a T (edited)\nhi"},
		{"compact", true, "T a hi (edited)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg.UI.MessagesList.Compact = test.compact

			var got strings.Builder
			for _, span := range ml.renderMessage(message, nil, tui.Style{}, false) {
				got.WriteString(span.Text)
			}

			if got.String() != test.want {
				t.Fatalf("got = %q, want = %q", got.String(), test.want)
			}
		})
		t.Run(test.name+" spaces around dimmed text in the base style", func(t *testing.T) {
			cfg.UI.MessagesList.Compact = test.compact
			for _, span := range ml.renderMessage(message, nil, tui.Style{}, false) {
				if span.Style.HasDim() && strings.TrimSpace(span.Text) != span.Text {
					t.Fatalf("dimmed span %q has spaces around it", span.Text)
				}
			}
		})
	}
}

func TestModelShouldGroup(t *testing.T) {
	start := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	message := func(author string, after time.Duration, typ discord.MessageType) discord.Message {
		return discord.Message{Type: typ, Author: discord.User{ID: 2, Username: author}, Timestamp: discord.NewTimestamp(start.Add(after)), Content: "hi"}
	}
	previous := message("a", 0, discord.DefaultMessage)
	for _, tt := range []struct {
		name     string
		previous discord.Message
		message  discord.Message
		want     bool
	}{
		{"same author", previous, message("a", 10*time.Minute, discord.DefaultMessage), true},
		{"after a reply", message("a", 0, discord.InlinedReplyMessage), message("a", time.Minute, discord.DefaultMessage), true},
		{"over the interval", previous, message("a", 10*time.Minute+time.Second, discord.DefaultMessage), false},
		{"renamed webhook", previous, message("b", time.Minute, discord.DefaultMessage), false},
		{"reply", previous, message("a", time.Minute, discord.InlinedReplyMessage), false},
		{"after a system message", message("a", 0, discord.ChannelPinnedMessage), message("a", time.Minute, discord.DefaultMessage), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := newTestModel(t, testConfig(t)).shouldGroup(&tt.previous, tt.message); got != tt.want {
				t.Fatalf("grouped = %v, want %v", got, tt.want)
			}
		})
	}

	cfg := testConfig(t)
	cfg.Timestamps.Enabled = false
	ml := newTestModel(t, cfg)
	t.Run("disabled", func(t *testing.T) {
		if cfg.UI.MessagesList.Grouping.Enabled = false; ml.shouldGroup(&previous, previous) {
			t.Fatal("grouped while disabled")
		}
	})

	cfg.UI.MessagesList.Grouping.Enabled = true
	// Newest first, as Discord sends them.
	ml.setMessages([]discord.Message{message("a", 2*time.Minute, discord.DefaultMessage), message("a", time.Minute, discord.DefaultMessage), previous})
	newest := ml.messageIndex(len(ml.items), -1)
	t.Run("only the first has a header", func(t *testing.T) {
		if !ml.items[newest].grouped || ml.items[ml.messageIndex(-1, 1)].grouped {
			t.Fatal("only the first message has a header")
		}
	})
	t.Run("grouped content", func(t *testing.T) {
		if got := ml.renderMessage(ml.items[newest].message, nil, tui.Style{}, true); len(got) != 1 || got[0].Text != "hi" {
			t.Fatalf("grouped message = %v, want only its content", got)
		}
	})
	t.Run("delete the first", func(t *testing.T) {
		ml.buildItem(ml.messageIndex(newest, -1))
		ml.deleteMessage(ml.messageIndex(-1, 1))
		if first := ml.messageIndex(-1, 1); ml.items[first].grouped || ml.items[first].view != nil {
			t.Fatal("deleting the first message did not give the next its header")
		}
	})
	t.Run("add", func(t *testing.T) {
		if ml.AddMessage(message("a", 3*time.Minute, discord.DefaultMessage)); !ml.items[len(ml.items)-1].grouped {
			t.Fatal("added message was not grouped")
		}
	})
}

func TestModelUpdate(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	ml.SetFocused(true)
	messages := make([]discord.Message, 20)
	for i := range messages {
		messages[i] = discord.Message{ID: discord.MessageID(len(messages) + 10 - i), Content: "hi"}
	}
	ml.setMessages(messages)
	area := tui.Rectangle{Width: 40, Height: 5}
	up := tui.MouseMsg{EventMouse: tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone), Action: tui.MouseScrollUp}
	// scrollUp scrolls up a row and returns what fetching older messages that does, and whether it scrolled.
	scrollUp := func() (tui.Cmd, bool) {
		t.Helper()
		msg, ok := ml.listView().Handle(up, area).(scrollMsg)
		if !ok {
			return nil, false
		}
		var cmd tui.Cmd
		*ml, cmd = ml.Update(msg)
		return cmd, true
	}
	// scrollToTop scrolls to the top and returns how many times older messages were fetched on the way.
	scrollToTop := func() (fetches int) {
		t.Helper()
		for {
			cmd, scrolled := scrollUp()
			if !scrolled {
				return fetches
			}
			if cmd != nil {
				fetches++
			}
		}
	}

	t.Run("scrolling to the top fetches once", func(t *testing.T) {
		if fetches := scrollToTop(); fetches != 1 {
			t.Fatalf("scrolling to the top fetched %d times, want once", fetches)
		}
	})
	t.Run("at the top", func(t *testing.T) {
		if cmd, _ := scrollUp(); cmd != nil {
			t.Fatal("older messages were fetched again while at the top")
		}
	})

	older := []discord.Message{{ID: 1, Content: "hi"}, {ID: 2, Content: "hi"}}
	*ml, _ = ml.Update(olderMessagesLoadedMsg{channelID: 1, older: older})
	t.Run("after older messages", func(t *testing.T) {
		if cmd, _ := scrollUp(); cmd != nil {
			t.Fatal("older messages were fetched again before scrolling to the new top")
		}
		if ml.scrollState.Viewport().AbsoluteOffsetReversed().Y == 0 {
			t.Fatal("the older messages were scrolled into view rather than added above it")
		}
	})
	t.Run("scrolling to the new top fetches once", func(t *testing.T) {
		if fetches := scrollToTop(); fetches != 1 {
			t.Fatalf("scrolling to the new top fetched %d times, want once", fetches)
		}
	})
}

func TestModelReactionsView(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	custom := discord.Emoji{ID: 5, Name: "cat"}
	reactions := []discord.Reaction{{Count: 2, Emoji: discord.Emoji{Name: "ok"}}, {Count: 1, Emoji: custom}}
	draw := func() string {
		screen, err := tcell.NewTerminfoScreenFromTty(vt.NewMockTerm(vt.MockOptSize{X: 20, Y: 1}))
		if err != nil {
			t.Fatal(err)
		}
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		defer screen.Fini()

		ml.reactionsView(reactions).Draw(screen, tui.Rectangle{Width: 20, Height: 1})
		var row strings.Builder
		for x := range 20 {
			str, _, _ := screen.Get(x, 0)
			row.WriteString(str)
		}
		return strings.TrimRight(row.String(), " ")
	}

	t.Run("names", func(t *testing.T) {
		if got := draw(); got != "ok 2 • cat 1" {
			t.Fatalf("reactions = %q", got)
		}
	})
	t.Run("emoji image", func(t *testing.T) {
		ml.images = map[discord.URL]tuiimage.Widget{emojis.URLFromID(custom.ID): tuiimage.New(image.NewAlpha(image.Rect(0, 0, 2, 2))).Width(2)}
		// The blank image takes two cells before the count.
		if got := draw(); got != "ok 2 •    1" {
			t.Fatalf("reactions = %q", got)
		}
	})
	t.Run("with name", func(t *testing.T) {
		ml.cfg.UI.MessagesList.Reactions.ShowEmojiName = true
		if got := draw(); got != "ok 2 •    cat 1" {
			t.Fatalf("reactions = %q", got)
		}
	})
	t.Run("with colons", func(t *testing.T) {
		ml.cfg.UI.MessagesList.Reactions.ColonsAroundName = true
		if got := draw(); got != "ok 2 •    :cat: 1" {
			t.Fatalf("reactions = %q", got)
		}
	})
}

func TestModelCanReact(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	t.Run("dm", func(t *testing.T) {
		if ml.channel = (&discord.Channel{ID: 1, Type: discord.DirectMessage}); !ml.canReact() {
			t.Error("cannot react in a DM")
		}
	})
	t.Run("without permissions", func(t *testing.T) {
		// The permissions in the guild are unknown.
		if ml.channel = (&discord.Channel{ID: 2, GuildID: 3}); ml.canReact() {
			t.Error("can react without the permissions")
		}
	})
}

func TestModelDrawAuthor(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	const guild, user, role = 7, 8, 9
	if err := ml.state.Cabinet.RoleSet(guild, &discord.Role{ID: role, Color: 0x00ff00, Position: 1}, false); err != nil {
		t.Fatal(err)
	}
	if err := ml.state.Cabinet.MemberSet(guild, &discord.Member{User: discord.User{ID: user}, Nick: "nick", RoleIDs: []discord.RoleID{role}}, false); err != nil {
		t.Fatal(err)
	}
	message := discord.Message{GuildID: guild, Author: discord.User{ID: user, Username: "user"}}
	green, red := tcell.NewHexColor(0x00ff00), tcell.NewHexColor(0xff0000)

	// evidence: witnesses
	for _, test := range []struct {
		name  string
		color string
		style tui.Style
		want  tui.Style
	}{
		{"role color over the style's", "role", tcell.StyleDefault.Foreground(red).Dim(true), tcell.StyleDefault.Foreground(green).Dim(true)},
		{"the style's color", "style", tcell.StyleDefault.Foreground(red), tcell.StyleDefault.Foreground(red)},
		{"regular", "style", tcell.StyleDefault, tcell.StyleDefault},
	} {
		t.Run(test.name, func(t *testing.T) {
			ml.cfg.Theme.MessagesList.AuthorColor = test.color
			ml.cfg.Theme.MessagesList.AuthorStyle = config.Style{Style: test.style}
			var builder markdown.Builder
			ml.drawAuthor(&builder, message, tcell.StyleDefault)
			spans := builder.Finish()
			if len(spans) != 1 || spans[0].Text != "nick" || spans[0].Style != test.want {
				t.Fatalf("spans = %+v, want nick in %v", spans, test.want)
			}
		})
	}
}

func TestModelAlternate(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	ml.cfg.UI.MessagesList.Grouping.Enabled = false
	ml.cfg.UI.MessagesList.AlternateEvery = 2
	ml.cfg.Theme.MessagesList.AlternateStyle = config.Style{Style: tcell.StyleDefault.Background(tcell.ColorBlue)}
	day := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	message := func(id discord.MessageID) discord.Message {
		return discord.Message{ID: id, Timestamp: discord.NewTimestamp(day.Add(time.Duration(id) * time.Minute))}
	}
	// shades returns whether each message, oldest first, takes the alternate style.
	shades := func() map[discord.MessageID]bool {
		got := map[discord.MessageID]bool{}
		for i := range ml.items {
			if item := &ml.items[i]; item.separator == nil {
				got[item.message.ID] = ml.alternate(item)
			}
		}
		return got
	}

	ml.setMessages([]discord.Message{message(13), message(12), message(11), message(10)})
	before := shades()
	if before[10] || !before[11] || before[12] || !before[13] {
		t.Fatalf("shades = %v, want every second message", before)
	}
	t.Run("a new message continues the pattern", func(t *testing.T) {
		ml.AddMessage(message(14))
		if got := shades(); got[14] {
			t.Fatalf("message 14 shaded after 13, which is: %v", got)
		}
	})
	t.Run("older messages leave those shown as they were", func(t *testing.T) {
		ml.prependOlder(olderMessagesLoadedMsg{channelID: ml.channel.ID, older: []discord.Message{message(7), message(8), message(9)}})
		got := shades()
		for id, shaded := range before {
			if got[id] != shaded {
				t.Fatalf("message %d changed shade: %v", id, got)
			}
		}
		if got[9] == got[10] {
			t.Fatalf("message 9 has the shade of 10: %v", got)
		}
	})
	t.Run("the selected card shows the selection, not the alternate tint", func(t *testing.T) {
		ml.cfg.Theme.MessagesList.SelectedMessageStyle = config.Style{Style: tcell.StyleDefault.Background(tcell.ColorRed)}
		screen, err := tcell.NewTerminfoScreenFromTty(vt.NewMockTerm(vt.MockOptSize{X: 20, Y: 1}))
		if err != nil {
			t.Fatal(err)
		}
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		defer screen.Fini()
		for i := range ml.items {
			if item := &ml.items[i]; item.separator == nil && ml.alternate(item) {
				ml.cursor = i
				break
			}
		}
		// As the list draws its selected item.
		selected := &tint.Screen{Screen: screen, Style: ml.cfg.Theme.MessagesList.SelectedMessageStyle.Style}
		selected.FillArea(0, 0, 20, 1, ' ', tcell.StyleDefault)
		ml.buildItem(ml.cursor).Draw(selected, tui.Rectangle{Width: 20, Height: 1})
		if _, style, _ := screen.Get(19, 0); style.GetBackground() != tcell.ColorRed {
			t.Fatalf("background = %v, want the selection's", style.GetBackground())
		}
	})
	t.Run("off at 0", func(t *testing.T) {
		ml.cfg.UI.MessagesList.AlternateEvery = 0
		for id, shaded := range shades() {
			if shaded {
				t.Fatalf("message %d shaded", id)
			}
		}
	})
}

func TestModelDrawReply(t *testing.T) {
	ml := newTestModel(t, testConfig(t))
	reply := tcell.StyleDefault.Italic(true).Background(tcell.ColorNavy)
	ml.cfg.Theme.MessagesList.ReplyStyle = config.Style{Style: reply}
	var builder markdown.Builder
	ml.drawReply(&builder, discord.Message{}, tcell.StyleDefault)
	spans := builder.Finish()
	if len(spans) == 0 || spans[0].Style != reply {
		t.Fatalf("indicator = %+v, want it in reply_style", spans)
	}
	if last := spans[len(spans)-1]; last.Text != "Original message was deleted" || last.Style != reply {
		t.Fatalf("deleted note = %+v, want it in reply_style", last)
	}
}

func TestModelDrawAttachments(t *testing.T) {
	img := discord.Attachment{Filename: "a.png", URL: "https://a", ContentType: "image/png", Width: 2, Height: 1}
	file := discord.Attachment{Filename: "b.txt", URL: "https://b"}
	for _, tt := range []struct {
		name                     string
		links, filename, preview bool
		want                     string
	}{
		{"defaults", false, false, true, "\nb.txt"},
		{"filename", false, true, true, "\na.png\nb.txt"},
		{"links", true, false, true, "\nhttps://a\nb.txt:\nhttps://b"},
		{"links and filename", true, true, true, "\na.png:\nhttps://a\nb.txt:\nhttps://b"},
		{"no preview", false, false, false, "\na.png\nb.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ml := newTestModel(t, testConfig(t))
			cfg := &ml.cfg.UI.MessagesList.Attachments
			cfg.ShowLinks, cfg.ShowFilename, cfg.Preview = tt.links, tt.filename, tt.preview
			var builder markdown.Builder
			ml.drawAttachments(&builder, []discord.Attachment{img, file}, tcell.StyleDefault)
			var got strings.Builder
			for _, span := range builder.Finish() {
				got.WriteString(span.Text)
			}
			if got.String() != tt.want {
				t.Fatalf("attachments = %q, want %q", got.String(), tt.want)
			}
		})
	}
}
