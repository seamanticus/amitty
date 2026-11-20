package memberstree

import (
	"slices"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen/states/member"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/ui/chat/profile"
)

// currentMsg makes a node of the tree the current one.
type currentMsg struct {
	current *tree.Node
}

// OpenDMMsg asks to open your DM with UserID, picked in the tree.
type OpenDMMsg struct{ UserID discord.UserID }

// currentMember returns the member highlighted, if a member is.
func (m Model) currentMember() (memberRef, bool) {
	if m.current == nil {
		return memberRef{}, false
	}
	ref, ok := m.current.Reference().(memberRef)
	return ref, ok
}

// profile returns the message that shows the profile of the member userID, if they are still known.
func (m Model) profile(userID discord.UserID) (profile.ShowMsg, bool) {
	channel := m.channel
	if channel == nil {
		return profile.ShowMsg{}, false
	}
	if channel.GuildID.IsValid() {
		member, err := m.state.Cabinet.Member(channel.GuildID, userID)
		if err != nil {
			return profile.ShowMsg{}, false
		}
		return profile.ShowMsg{GuildID: channel.GuildID, Member: *member}, true
	}
	recipients := channel.DMRecipients
	if me, err := m.state.Cabinet.Me(); err == nil {
		recipients = append(slices.Clone(recipients), *me)
	}
	index := slices.IndexFunc(recipients, func(user discord.User) bool { return user.ID == userID })
	if index < 0 {
		return profile.ShowMsg{}, false
	}
	return profile.ShowMsg{GuildID: discord.NoGuild(), Member: discord.Member{User: recipients[index]}}, true
}

// selectedMsg is sent when the user selects a node.
type selectedMsg struct {
	node *tree.Node
}

// requestSelectedMembers requests the chunk of the member list after the selected member, so that the members below it load before they are reached.
func (m Model) requestSelectedMembers() tui.Cmd {
	node := m.current
	if node == nil {
		return nil
	}
	ref, ok := node.Reference().(memberRef)
	if !ok || ref.index < 0 {
		return nil
	}
	return m.requestMembers(member.ChunkFromIndex(ref.index) + 1)
}

// requestMembers requests the member list of the selected guild channel up to chunk.
func (m Model) requestMembers(chunk int) tui.Cmd {
	channel := m.channel
	if !m.Shown() || !channel.GuildID.IsValid() {
		return nil
	}
	state := m.state
	return cmd.Task(func() tui.Msg {
		state.MemberState.RequestMemberList(channel.GuildID, channel.ID, chunk)
		return nil
	})
}
