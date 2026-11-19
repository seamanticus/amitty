package memberstree

import (
	"github.com/seamanticus/amitty/internal/ningen/states/member"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/tree"
)

// currentMsg makes a node of the tree the current one.
type currentMsg struct {
	current *tree.Node
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
