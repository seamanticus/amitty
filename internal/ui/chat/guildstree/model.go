package guildstree

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/ningen/states/read"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/tui/viewport"
	"github.com/seamanticus/amitty/internal/ui"
	uitree "github.com/seamanticus/amitty/internal/ui/tree"
)

type dmNode struct{}

const (
	dmNodeName        = "DMs"
	maxMentions       = 99
	mentionsFormat    = " (%d)"
	maxMentionsFormat = " (%d+)"
)

type Model struct {
	root        *tree.Node
	current     *tree.Node
	scrollState viewport.ScrollState

	cfg   *config.Config
	state *ningen.State

	// guildNodes and channelNodes index the nodes for frequent event handlers (read updates, picker navigation).
	// They mirror the current rendered tree and are rebuilt on READY before nodes are added.
	guildNodes   map[discord.GuildID]*tree.Node
	channelNodes map[discord.ChannelID]*tree.Node
	dmRootNode   *tree.Node
	// folders holds each folder node's folder, so its unread state can be taken from its guilds.
	folders map[*tree.Node]gateway.GuildFolder

	// focused reports whether the tree takes keys.
	focused bool
}

func NewModel(cfg *config.Config, state *ningen.State) Model {
	return Model{
		root:         tree.NewNode(""),
		cfg:          cfg,
		state:        state,
		guildNodes:   make(map[discord.GuildID]*tree.Node),
		channelNodes: make(map[discord.ChannelID]*tree.Node),
		folders:      make(map[*tree.Node]gateway.GuildFolder),
	}
}

// CurrentNode returns the selected node, or nil for none.
func (m Model) CurrentNode() *tree.Node {
	return m.current
}

// SetFocused sets whether the tree takes keys.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

func (m *Model) reset() *tree.Node {
	// Keep allocated map capacity; READY can rebuild often during reconnects.
	clear(m.guildNodes)
	clear(m.channelNodes)
	clear(m.folders)
	m.dmRootNode = tree.NewNode(m.dmNodeText()).
		SetReference(dmNode{}).
		SetExpandable(true).
		SetExpanded(false)
	setNodeNameStyle(m.dmRootNode, m.dmNodeStyle())
	return m.root.ClearChildren().AddChild(m.dmRootNode)
}

func (m *Model) rebuild(event *gateway.ReadyEvent) {
	root := m.reset()
	guildsInFolders := make(map[discord.GuildID]bool)
	for _, folder := range event.UserSettings.GuildFolders {
		for _, guildID := range folder.GuildIDs {
			guildsInFolders[guildID] = true
		}
	}

	guildsByID := make(map[discord.GuildID]*gateway.GuildCreateEvent, len(event.Guilds))
	for index := range event.Guilds {
		guildsByID[event.Guilds[index].ID] = &event.Guilds[index]
	}

	positions := event.UserSettings.GuildPositions
	if len(positions) == 0 {
		positions = make([]discord.GuildID, 0, len(event.Guilds))
		for _, guild := range event.Guilds {
			positions = append(positions, guild.ID)
		}
	}

	for _, guildID := range positions {
		if guild := guildsByID[guildID]; guild != nil && !guildsInFolders[guildID] {
			m.createGuildNode(root, guild.Guild)
		}
	}

	for _, folder := range event.UserSettings.GuildFolders {
		// A guild in a folder of its own is shown as if it were in none.
		if folder.ID != 0 || len(folder.GuildIDs) != 1 {
			m.createFolderNode(folder, guildsByID)
			continue
		}

		if guild := guildsByID[folder.GuildIDs[0]]; guild != nil {
			m.createGuildNode(root, guild.Guild)
		}
	}

	m.setCurrentNode(root)
}

func (m Model) refreshReadState(event *read.UpdateEvent) {
	if event.GuildID.IsValid() {
		m.refreshGuildNode(event.GuildID)
	} else if m.dmRootNode != nil {
		m.refreshDMRoot()
	}

	node := m.findNodeByRef(event.ChannelID)
	if node == nil {
		return
	}
	channel, err := m.state.Cabinet.Channel(event.ChannelID)
	if err != nil {
		indication := m.state.ChannelIsUnread(event.ChannelID, ningen.UnreadOpts{IncludeMutedCategories: true})
		setNodeNameStyle(node, unreadStyle(indication))
		return
	}

	setNodeText(node, m.channelText(*channel))
	setNodeNameStyle(node, m.channelNodeStyle(*channel))
}

func (m Model) refreshGuildNode(guildID discord.GuildID) {
	node := m.findNodeByRef(guildID)
	if node == nil {
		return
	}

	if guild, err := m.state.Cabinet.Guild(guildID); err == nil {
		setNodeText(node, m.guildText(*guild))
	}

	setNodeNameStyle(node, m.guildNodeStyle(guildID))

	for folderNode, folder := range m.folders {
		if slices.Contains(folder.GuildIDs, guildID) {
			m.refreshFolderNode(folderNode, folder)
		}
	}
}

// refreshDMRoot sets the DM root's counter and unread style from the DMs shown.
func (m Model) refreshDMRoot() {
	setNodeText(m.dmRootNode, m.dmNodeText())
	setNodeNameStyle(m.dmRootNode, m.dmNodeStyle())
}

// refreshFolderNode sets a folder's counter and unread style from its guilds, keeping its color.
func (m Model) refreshFolderNode(node *tree.Node, folder gateway.GuildFolder) {
	name := "Folder"
	if folder.Name != "" {
		name = folder.Name
	}
	var mentions int
	indication := ningen.ChannelRead
	for _, guildID := range folder.GuildIDs {
		channels, _ := m.state.Cabinet.Channels(guildID)
		mentions += m.mentions(channels...)
		indication = max(indication, m.state.GuildIsUnread(guildID, ningen.GuildUnreadOpts{IncludeMutedCategories: true}))
	}
	style := unreadStyle(indication)
	if folder.Color != 0 {
		style = style.Foreground(tcell.NewHexColor(int32(folder.Color)))
	}
	setNodeText(node, name+mentionsLabel(mentions))
	setNodeNameStyle(node, style)
}

func (m Model) updateDMNodeStyle(userID discord.UserID) {
	channel, err := m.state.Cabinet.CreatePrivateChannel(userID)
	if err != nil {
		return
	}

	node, ok := m.channelNodes[channel.ID]
	if node == nil || !ok {
		return
	}
	setNodeNameStyle(node, m.channelNodeStyle(*channel))
}

func (m Model) moveDMToFront(channelID discord.ChannelID) {
	if m.dmRootNode == nil {
		return
	}

	node := m.channelNodes[channelID]
	children := m.dmRootNode.Children()
	if index := slices.Index(children, node); index > 0 {
		copy(children[1:index+1], children[:index])
		children[0] = node
	}
}

func (m Model) createFolderNode(folder gateway.GuildFolder, guildsByID map[discord.GuildID]*gateway.GuildCreateEvent) {
	folderNode := tree.NewNode("").SetExpanded(m.cfg.UI.GuildsTree.AutoExpandFolders)
	m.folders[folderNode] = folder
	m.refreshFolderNode(folderNode, folder)
	m.root.AddChild(folderNode)

	for _, guildID := range folder.GuildIDs {
		if guildEvent, ok := guildsByID[guildID]; ok {
			m.createGuildNode(folderNode, guildEvent.Guild)
		}
	}
}

func (m Model) guildNodeStyle(guildID discord.GuildID) tui.Style {
	indication := m.state.GuildIsUnread(guildID, ningen.GuildUnreadOpts{IncludeMutedCategories: true})
	return unreadStyle(indication)
}

func (m Model) channelNodeStyle(channel discord.Channel) tui.Style {
	unread := unreadStyle(m.state.ChannelIsUnread(channel.ID, ningen.UnreadOpts{IncludeMutedCategories: true}))
	if channel.Type != discord.DirectMessage || len(channel.DMRecipients) != 1 {
		return unread
	}

	recipient := channel.DMRecipients[0]
	presence, err := m.state.Cabinet.Presence(discord.NoGuild(), recipient.ID)
	if err != nil {
		return tui.MergeStyle(m.cfg.Theme.GuildsTree.StatusStyle(discord.OfflineStatus), unread)
	}

	return tui.MergeStyle(m.cfg.Theme.GuildsTree.StatusStyle(presence.Status), unread)
}

func (m Model) createGuildNode(parent *tree.Node, guild discord.Guild) {
	guildNode := tree.NewNode(m.guildText(guild)).
		SetReference(guild.ID).
		SetExpandable(true).
		SetExpanded(false).
		SetIndent(m.cfg.Sidebar.Indents.Guild)
	setNodeNameStyle(guildNode, m.guildNodeStyle(guild.ID))
	parent.AddChild(guildNode)
	m.guildNodes[guild.ID] = guildNode
}

// addJoinedGuild lists guild, joined since Ready, first after the DMs, as the web client does; a guild already listed is left in place.
func (m Model) addJoinedGuild(guild discord.Guild) {
	if _, ok := m.guildNodes[guild.ID]; ok {
		return
	}
	m.createGuildNode(m.root, guild)
	children := m.root.Children()
	last := len(children) - 1
	m.root.SetChildren(slices.Insert(children[:last], min(1, last), children[last]))
}

func (m Model) createChannelNode(parent *tree.Node, channel discord.Channel) {
	needsViewPermission := !slices.Contains([]discord.ChannelType{discord.DirectMessage, discord.GroupDM, discord.GuildCategory}, channel.Type)
	if needsViewPermission && !m.state.HasPermissions(channel.ID, discord.PermissionViewChannel) {
		return
	}

	indents := m.cfg.Sidebar.Indents
	channelNode := tree.NewNode(m.channelText(channel)).SetReference(channel.ID)
	setNodeNameStyle(channelNode, m.channelNodeStyle(channel))
	switch channel.Type.Kind() {
	case discord.DirectMessageKind:
		channelNode.SetIndent(indents.DM)
	case discord.GroupDMKind:
		channelNode.SetIndent(indents.GroupDM)
	case discord.CategoryKind:
		channelNode.SetIndent(indents.Category)
		channelNode.SetExpandable(true).SetExpanded(true)
	case discord.ForumKind:
		channelNode.SetIndent(indents.Forum)
		channelNode.SetExpandable(true).SetExpanded(false)
	case discord.ThreadKind, discord.GuildChannelKind, discord.UnknownChannelKind:
		channelNode.SetIndent(indents.Channel)
	}
	parent.AddChild(channelNode)
	m.channelNodes[channel.ID] = channelNode
}

// expandable reports whether selecting node expands or collapses it.
func expandable(node *tree.Node) bool {
	return len(node.Children()) > 0
}

// toggleFully expands node and everything under it, loading the channels of servers and the DMs not listed yet, or collapses it all when it is all expanded.
func (m Model) toggleFully(node *tree.Node) {
	if node == nil {
		return
	}
	if m.fullyExpanded(node) {
		node.CollapseAll()
		return
	}
	var expand func(*tree.Node)
	expand = func(n *tree.Node) {
		if len(n.Children()) == 0 {
			switch ref := n.Reference().(type) {
			case discord.GuildID:
				m.expandGuild(n, ref)
			case dmNode:
				m.expandPrivateChannels(n)
			}
		}
		if len(n.Children()) == 0 {
			return
		}
		n.Expand()
		for _, child := range n.Children() {
			expand(child)
		}
	}
	expand(node)
}

// fullyExpanded reports whether node and every node under it with children are expanded, and no server or DM list under it is still unloaded.
func (m Model) fullyExpanded(node *tree.Node) bool {
	all := true
	node.Walk(func(n, _ *tree.Node) bool {
		switch n.Reference().(type) {
		case discord.GuildID, dmNode:
			if len(n.Children()) == 0 {
				all = false
			}
		}
		if len(n.Children()) > 0 && !n.Expanded() {
			all = false
		}
		return all
	})
	return all
}

// setNodeNameStyle styles the name of node, which is the first span of its text.
func setNodeNameStyle(node *tree.Node, style tui.Style) {
	spans := node.Spans()
	spans[0].Style = style
	node.SetSpans(spans...)
}

// setNodeText sets the text of the name of node, keeping its style.
func setNodeText(node *tree.Node, name string) {
	spans := node.Spans()
	spans[0].Text = name
	node.SetSpans(spans...)
}

// channelText returns the name of channel with its unread mentions after it.
func (m Model) channelText(channel discord.Channel) string {
	text := ui.ChannelToString(channel, m.cfg.Icons, m.state)
	if channel.IsMessageRequest {
		text += requestLabel
	}
	return text + mentionsLabel(m.mentions(channel))
}

// guildText returns the name of guild with the unread mentions in its channels after it.
func (m Model) guildText(guild discord.Guild) string {
	channels, _ := m.state.Cabinet.Channels(guild.ID)
	return guild.Name + mentionsLabel(m.mentions(channels...))
}

// dmNodeText returns the name of the direct messages node with their unread mentions after it.
func (m Model) dmNodeText() string {
	channels, _ := ui.VisiblePrivateChannels(m.state, m.cfg.HideDeletedUsers)
	return dmNodeName + mentionsLabel(m.mentions(channels...))
}

// dmNodeStyle returns the unread style of the DM root: that of its most unread DM shown.
func (m Model) dmNodeStyle() tui.Style {
	channels, _ := ui.VisiblePrivateChannels(m.state, m.cfg.HideDeletedUsers)
	indication := ningen.ChannelRead
	for _, channel := range channels {
		indication = max(indication, m.state.ChannelIsUnread(channel.ID, ningen.UnreadOpts{IncludeMutedCategories: true}))
	}
	return unreadStyle(indication)
}

// mentions returns the number of unread mentions in channels.
func (m Model) mentions(channels ...discord.Channel) (total int) {
	for _, channel := range channels {
		if state := m.state.ReadState.ReadState(channel.ID); state != nil {
			total += state.MentionCount
		}
	}
	return total
}

// mentionsLabel returns what follows a name with count unread mentions, or nothing if there are none.
func mentionsLabel(count int) string {
	switch {
	case count == 0:
		return ""
	case count > maxMentions:
		return fmt.Sprintf(maxMentionsFormat, maxMentions)
	}
	return fmt.Sprintf(mentionsFormat, count)
}

func (m Model) createChannelNodes(node *tree.Node, channels []discord.Channel) {
	// Preserve exact ordering semantics:
	// 1) top-level non-categories (in input order),
	// 2) categories that have at least one child in the source slice (in input order),
	// 3) parented channels under already-created categories (in input order).
	//
	// We precompute parent presence once to avoid the O(n^2) category-child scan.
	hasChildByParentID := make(map[discord.ChannelID]struct{}, len(channels))
	for _, channel := range channels {
		if channel.ParentID.IsValid() {
			hasChildByParentID[channel.ParentID] = struct{}{}
		}
	}

	for _, channel := range channels {
		if channel.Type != discord.GuildCategory && !channel.ParentID.IsValid() {
			m.createChannelNode(node, channel)
		}
	}

	for _, channel := range channels {
		if _, hasChild := hasChildByParentID[channel.ID]; hasChild && channel.Type == discord.GuildCategory {
			m.createChannelNode(node, channel)
		}
	}

	for _, channel := range channels {
		if !channel.ParentID.IsValid() {
			continue
		}

		// Parent categories are inserted earlier in this function, so this lookup is O(1) and avoids per-channel subtree walks.
		if parent := m.channelNodes[channel.ParentID]; parent != nil {
			m.createChannelNode(parent, channel)
		}
	}
}

func (m *Model) collapseParentNode(node *tree.Node) {
	path := m.root.PathTo(node)
	if len(path) < 3 {
		return
	}
	parent := path[len(path)-2]
	parent.Collapse()
	m.setCurrentNode(parent)
}

// setCurrentNode selects node and scrolls to it.
func (m *Model) setCurrentNode(node *tree.Node) {
	m.current = node
	m.scrollState.ScrollToTarget()
}

// selectAdjacentUnread selects the next unread node, or the previous one if previous.
func (m *Model) selectAdjacentUnread(previous bool) {
	node := adjacentNode(m.root, m.current, m.isUnread, previous)
	if node == nil {
		return
	}

	// A guild or the direct messages without channels yet: load them and move on to the unread one.
	if _, ok := node.Reference().(discord.ChannelID); !ok {
		m.selectNode(node)
		if channel := adjacentNode(node, node, m.isUnread, previous); channel != nil {
			node = channel
		}
	}

	m.expandPathToNode(node)
	m.setCurrentNode(node)
}

// isUnread reports whether node is an unread channel, or a guild or the direct messages whose channels are not shown yet and has one.
func (m Model) isUnread(node *tree.Node) bool {
	if len(node.Children()) > 0 {
		return false
	}
	opts := ningen.UnreadOpts{IncludeMutedCategories: true}
	switch ref := node.Reference().(type) {
	case discord.GuildID:
		return m.state.GuildIsUnread(ref, ningen.GuildUnreadOpts{UnreadOpts: opts}) != ningen.ChannelRead
	case discord.ChannelID:
		return m.state.ChannelIsUnread(ref, opts) != ningen.ChannelRead
	case dmNode:
		channels, _ := ui.VisiblePrivateChannels(m.state, m.cfg.HideDeletedUsers)
		return slices.ContainsFunc(channels, func(channel discord.Channel) bool {
			return m.state.ChannelIsUnread(channel.ID, opts) != ningen.ChannelRead
		})
	}

	return false
}

// adjacentNode returns the nearest node under root matching match after current, or before it if previous, wrapping around, or nil if no other node matches.
func adjacentNode(root, current *tree.Node, match func(*tree.Node) bool, previous bool) *tree.Node {
	var nodes []*tree.Node
	after := 0
	root.Walk(func(node, _ *tree.Node) bool {
		if node == current {
			after = len(nodes)
		} else if match(node) {
			nodes = append(nodes, node)
		}
		return true
	})
	if len(nodes) == 0 {
		return nil
	}
	if previous {
		after += len(nodes) - 1
	}
	return nodes[after%len(nodes)]
}

func (m Model) findNodeByRef(ref any) *tree.Node {
	switch ref := ref.(type) {
	case discord.GuildID:
		return m.guildNodes[ref]
	case discord.ChannelID:
		return m.channelNodes[ref]
	case dmNode:
		return m.dmRootNode
	}

	// Fallback keeps this helper safe for non-indexed custom references.
	var found *tree.Node
	m.root.Walk(func(node, _ *tree.Node) bool {
		if node.Reference() == ref {
			found = node
		}

		return found == nil
	})

	return found
}

func (m Model) findNodeByChannelID(channelID discord.ChannelID) *tree.Node {
	channel, err := m.state.Cabinet.Channel(channelID)
	if err != nil {
		slog.Error("failed to get channel", "channel_id", channelID, "err", err)
		return nil
	}

	var ref any
	if guildID := channel.GuildID; guildID.IsValid() {
		ref = guildID
	} else {
		ref = dmNode{}
	}
	if parent := m.findNodeByRef(ref); parent != nil && len(parent.Children()) == 0 {
		m.selectNode(parent)
	}

	return m.findNodeByRef(channelID)
}

func (m Model) expandPathToNode(node *tree.Node) {
	if node == nil {
		return
	}
	for _, ancestor := range m.root.PathTo(node) {
		ancestor.Expand()
	}
}

func unreadStyle(indication ningen.UnreadIndication) tui.Style {
	var style tui.Style
	switch indication {
	case ningen.ChannelRead:
		style = style.Dim(true)
	case ningen.ChannelMentioned:
		style = style.Underline(true)
		fallthrough
	case ningen.ChannelUnread:
		style = style.Bold(true)
	}
	return style
}

var _ tui.Model[Model] = Model{}

func (Model) Init() tui.Cmd { return nil }

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case *gateway.ReadyEvent:
		m.rebuild(msg)
		return m, nil
	case *state.GuildJoinEvent:
		m.addJoinedGuild(msg.Guild)
		return m, nil
	case *gateway.MessageCreateEvent:
		if !msg.GuildID.IsValid() {
			m.moveDMToFront(msg.ChannelID)
		}
		return m, nil
	case *gateway.PresenceUpdateEvent:
		m.updateDMNodeStyle(msg.User.ID)
		return m, nil
	case *read.UpdateEvent:
		m.refreshReadState(msg)
		return m, nil
	case NavigateMsg:
		return m, m.navigate(msg.ChannelID)
	case ShowGuildMsg:
		m.showGuild(msg.Guild)
		return m, nil
	case requestsChangedMsg:
		m.RefreshPrivateChannels()
		return m, nil
	case currentMsg:
		m.current = msg.current
		m.scrollState.ScrollToTarget()
		return m, nil
	case uitree.ScrollMsg:
		m.scrollState = msg.ScrollState
		if msg.Msg != nil {
			return m.Update(msg.Msg)
		}
		return m, nil
	case selectedMsg:
		m.setCurrentNode(msg.node)
		return m, m.selectNode(msg.node)
	case tui.KeyMsg:
		keybinds := m.cfg.Keybinds.GuildsTree
		switch {
		case keybind.Matches(msg, keybinds.ToggleExpandAll.Keybind):
			m.toggleFully(m.current)
			return m, nil
		case keybind.Matches(msg, keybinds.CollapseAll.Keybind):
			for _, node := range m.root.Children() {
				node.CollapseAll()
			}

			return m, nil
		case keybind.Matches(msg, keybinds.CollapseParentNode.Keybind):
			m.collapseParentNode(m.current)
			return m, nil
		case keybind.Matches(msg, keybinds.SelectPreviousUnread.Keybind, keybinds.SelectNextUnread.Keybind):
			m.selectAdjacentUnread(keybind.Matches(msg, keybinds.SelectPreviousUnread.Keybind))
			return m, nil
		case keybind.Matches(msg, keybinds.YankID.Keybind):
			return m, uitree.YankID(m.current)
		case keybind.Matches(msg, keybinds.AcceptRequest.Keybind, keybinds.RejectRequest.Keybind):
			if channel, ok := m.currentRequest(); ok {
				return m, m.confirmRequest(channel, keybind.Matches(msg, keybinds.AcceptRequest.Keybind))
			}
			return m, nil
		case keybind.Matches(msg, keybinds.JoinGuild.Keybind):
			return m, cmd.Task(func() tui.Msg { return JoinMsg{} })
		case keybind.Matches(msg, keybinds.MarkRead.Keybind):
			return m, m.confirmMarkRead(m.current)
		}
	}

	return m, nil
}

// View shows the tree in a box titled Guilds.
func (m Model) View() tui.Widget {
	onChange := func(current *tree.Node) tui.Msg { return currentMsg{current} }
	onSelect := func(n *tree.Node) tui.Msg { return selectedMsg{node: n} }
	return uitree.New(
		m.root,
		m.current,
		m.scrollState,
		m.cfg,
		m.cfg.Theme.GuildsTree.TreeThemeConfig,
		m.cfg.UI.GuildsTree.Graphics,
		m.cfg.Keybinds.GuildsTree.TreeKeybinds,
		m.focused,
		onChange,
		onSelect,
	).
		Title(ui.Label("Guilds", m.cfg.Theme.Title, m.focused))
}
