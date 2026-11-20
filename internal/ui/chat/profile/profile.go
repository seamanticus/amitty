// Package profile shows what amitty knows of a member, from the state it already holds, with no request of its own.
package profile

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/images"
)

// avatarRows is the height of the avatar in the panel.
const avatarRows = 5

// ShowMsg asks to show the profile of Member, of the guild GuildID or of a DM for none.
type ShowMsg struct {
	GuildID discord.GuildID
	Member  discord.Member
}

// ClosedMsg reports that the panel was closed.
type ClosedMsg struct{}

type Model struct {
	cfg     *config.Config
	state   *ningen.State
	guildID discord.GuildID
	member  discord.Member
	images  images.ChangedMsg
}

// New returns the panel for msg and the command that loads the avatar.
func New(cfg *config.Config, state *ningen.State, msg ShowMsg) (Model, tui.Cmd) {
	m := Model{cfg: cfg, state: state, guildID: msg.GuildID, member: msg.Member}
	if !cfg.UI.MessagesList.Avatars.Preview {
		return m, nil
	}
	request := images.RequestedMsg{Avatars: map[discord.URL]uint{m.avatarURL(): avatarRows}}
	return m, cmd.Task(func() tui.Msg { return request })
}

// avatarURL differs from the messages' by its size, so the two load apart, each at its own height.
func (m Model) avatarURL() discord.URL {
	return m.member.User.AvatarURLWithType(discord.PNGImage) + "?size=256"
}

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case images.ChangedMsg:
		m.images = msg
	case tui.KeyMsg:
		keybinds := m.cfg.Keybinds.Profile
		switch {
		case keybind.Matches(msg, keybinds.YankID.Keybind):
			return m, ui.WriteClipboard(m.member.User.ID.String())
		case keybind.Matches(msg, keybinds.Close.Keybind):
			return m, cmd.Task(func() tui.Msg { return ClosedMsg{} })
		}
	}
	return m, nil
}

func (m Model) ShortHelp() []keybind.Keybind  { return m.cfg.Keybinds.Profile.ShortHelp() }
func (m Model) FullHelp() [][]keybind.Keybind { return m.cfg.Keybinds.Profile.FullHelp() }

// View shows the avatar beside the names and status, and the details below.
func (m Model) View() tui.Widget {
	var header tui.Widget = column.New(text.Rich(m.header()...)).Height(layout.Shrink)
	// A blank image holds the avatar's place while it loads; one that failed to load is 0 wide, and the names take the row.
	if avatar, ok := m.images[m.avatarURL()]; ok {
		if width, _ := avatar.Size(); width != layout.Fixed(0) {
			header = row.New(avatar, header).Spacing(1).Height(layout.Shrink)
		}
	}
	body := column.New(header, text.New(""), text.Rich(m.details()...).Wrapping(text.WrappingWordOrGlyph))
	return ui.Box(body, m.cfg, true).Title(ui.Label("Profile", m.cfg.Theme.Title, true))
}

// header returns the name in the member's color, the server tag, the username, the status and the activities.
func (m Model) header() []text.Span {
	user := m.member.User
	theme := m.cfg.Theme.MessagesList
	nameStyle := theme.AuthorStyle.Style
	if color, ok := state.MemberColor(&m.member, m.role); ok {
		nameStyle = nameStyle.Foreground(tcell.NewHexColor(int32(color)))
	}
	muted := theme.Embeds.FooterStyle.Style

	spans := []text.Span{{Text: cmp.Or(m.member.Nick, user.DisplayName, user.Username), Style: nameStyle}}
	if tag := user.ShownTag(); tag != "" {
		spans = append(spans, text.Span{Text: " [" + tag + "]", Style: muted})
	}
	if user.Bot {
		spans = append(spans, text.Span{Text: " BOT", Style: theme.MentionStyle.Style})
	}
	spans = append(spans, text.Span{Text: "\n" + user.Tag(), Style: muted})

	presence, err := m.state.Cabinet.Presence(m.guildID, user.ID)
	status := discord.OfflineStatus
	if err == nil {
		status = presence.Status
	}
	spans = append(spans,
		text.Span{Text: "\n"},
		text.Span{Text: "● ", Style: m.cfg.Theme.MembersTree.StatusStyle(status)},
		text.Span{Text: statusName(status)},
	)
	if err != nil {
		return spans
	}
	if clients := clientNames(presence.ClientStatus); clients != "" {
		spans = append(spans, text.Span{Text: " on " + clients, Style: muted})
	}
	for _, activity := range presence.Activities {
		if line := activityLine(activity); line != "" {
			spans = append(spans, text.Span{Text: "\n" + line})
		}
	}
	return spans
}

// details returns a line per known fact, each a label and its value.
func (m Model) details() []text.Span {
	user := m.member.User
	fields := m.cfg.Theme.MessagesList.Embeds
	var spans []text.Span
	field := func(label string, value ...text.Span) {
		if len(spans) != 0 {
			spans = append(spans, text.Span{Text: "\n"})
		}
		spans = append(spans, text.Span{Text: label + ": ", Style: fields.FieldNameStyle.Style})
		spans = append(spans, value...)
	}
	plain := func(value string) text.Span { return text.Span{Text: value, Style: fields.FieldValueStyle.Style} }

	if roles := m.roles(); len(roles) != 0 {
		field("Roles", roles...)
	}
	member := m.member
	if member.Joined.IsValid() {
		field("Member since", plain(m.date(member.Joined.Time())))
	}
	if member.BoostedSince.IsValid() {
		field("Boosting since", plain(m.date(member.BoostedSince.Time())))
	}
	if member.CommunicationDisabledUntil.IsValid() && member.CommunicationDisabledUntil.Time().After(time.Now()) {
		field("Timed out until", plain(m.date(member.CommunicationDisabledUntil.Time())))
	}
	if member.IsPending {
		field("Pending", plain("has not passed membership screening"))
	}
	field("Account created", plain(m.date(user.CreatedAt())))
	if badges := badgeNames(user.PublicFlags); badges != "" {
		field("Badges", plain(badges))
	}
	if relationship, ok := m.state.RelationshipState.FullRelationship(user.ID); ok {
		if line := m.relationshipLine(relationship); line != "" {
			field("Relationship", plain(line))
		}
		if nickname := relationship.Nickname; nickname != nil && *nickname != "" {
			field("Friend nickname", plain(*nickname))
		}
	}
	if note := m.state.NoteState.Note(user.ID); note != "" {
		field("Note", plain(note))
	}
	field("ID", plain(user.ID.String()))
	return spans
}

func (m Model) role(id discord.RoleID) *discord.Role {
	role, _ := m.state.Cabinet.Role(m.guildID, id)
	return role
}

// roles returns the member's roles, highest first, each in its color.
func (m Model) roles() []text.Span {
	var roles []discord.Role
	for _, id := range m.member.RoleIDs {
		if role := m.role(id); role != nil {
			roles = append(roles, *role)
		}
	}
	slices.SortFunc(roles, func(a, b discord.Role) int { return cmp.Compare(b.Position, a.Position) })
	var spans []text.Span
	for i, role := range roles {
		if i > 0 {
			spans = append(spans, text.Span{Text: ", "})
		}
		var style tui.Style
		if role.Color != 0 {
			style = style.Foreground(tcell.NewHexColor(int32(role.Color)))
		}
		spans = append(spans, text.Span{Text: role.Name, Style: style})
	}
	return spans
}

func (m Model) date(t time.Time) string {
	return t.Local().Format(m.cfg.DateSeparator.Format)
}

func (m Model) relationshipLine(relationship discord.Relationship) string {
	switch relationship.Type {
	case discord.FriendRelationship:
		if relationship.Since.IsValid() {
			return "friends since " + m.date(relationship.Since.Time())
		}
		return "friends"
	case discord.BlockedRelationship:
		return "blocked"
	case discord.IncomingFriendRequest:
		return "sent you a friend request"
	case discord.SentFriendRequest:
		return "friend request sent"
	}
	// Types newer than amitty, or none.
	return ""
}

func statusName(status discord.Status) string {
	switch status {
	case discord.OnlineStatus:
		return "Online"
	case discord.IdleStatus:
		return "Idle"
	case discord.DoNotDisturbStatus:
		return "Do not disturb"
	case discord.InvisibleStatus, discord.OfflineStatus, discord.UnknownStatus:
		return "Offline"
	}
	// Statuses newer than amitty.
	return string(status)
}

// clientNames returns the platforms the user is connected on, such as "desktop, mobile".
func clientNames(status discord.ClientStatus) string {
	var names []string
	for _, client := range []struct {
		name   string
		status discord.Status
	}{{"desktop", status.Desktop}, {"mobile", status.Mobile}, {"web", status.Web}} {
		if client.status != "" && client.status != discord.OfflineStatus {
			names = append(names, client.name)
		}
	}
	return strings.Join(names, ", ")
}

// activityLine returns an activity as Discord words it, such as "Listening to Spotify".
func activityLine(activity discord.Activity) string {
	switch activity.Type {
	case discord.CustomActivity:
		return strings.TrimSpace(emojiText(activity.Emoji) + " " + activity.State)
	case discord.GameActivity:
		return "Playing " + activity.Name
	case discord.StreamingActivity:
		return "Streaming " + cmp.Or(activity.Details, activity.Name)
	case discord.ListeningActivity:
		return "Listening to " + activity.Name
	case discord.WatchingActivity:
		return "Watching " + activity.Name
	case discord.CompetingActivity:
		return "Competing in " + activity.Name
	}
	// Types newer than amitty.
	return activity.Name
}

func emojiText(emoji *discord.Emoji) string {
	if emoji == nil {
		return ""
	}
	if emoji.ID.IsValid() {
		return ":" + emoji.Name + ":"
	}
	return emoji.Name
}

// badges are the public flags Discord shows as badges, in its order.
var badges = []struct {
	flag discord.UserFlags
	name string
}{
	{discord.Employee, "Staff"},
	{discord.Partner, "Partner"},
	{discord.CertifiedModerator, "Moderator Programs Alumni"},
	{discord.HypeSquadEvents, "HypeSquad Events"},
	{discord.HouseBravery, "HypeSquad Bravery"},
	{discord.HouseBrilliance, "HypeSquad Brilliance"},
	{discord.HouseBalance, "HypeSquad Balance"},
	{discord.BugHunterLvl1, "Bug Hunter"},
	{discord.BugHunterLvl2, "Bug Hunter (gold)"},
	{discord.VerifiedBotDeveloper, "Early Verified Bot Developer"},
	{discord.EarlySupporter, "Early Supporter"},
}

func badgeNames(flags discord.UserFlags) string {
	var names []string
	for _, badge := range badges {
		if flags&badge.flag != 0 {
			names = append(names, badge.name)
		}
	}
	return strings.Join(names, ", ")
}
