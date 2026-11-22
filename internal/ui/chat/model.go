package chat

import (
	"cmp"
	"context"
	"fmt"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"log/slog"
	"slices"
	"time"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/session"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/state/store/defaultstore"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	clientgateway "github.com/seamanticus/amitty/internal/gateway"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/backdrop"
	"github.com/seamanticus/amitty/internal/tui/center"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	tuiimage "github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/inert"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/picker"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/stack"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/autocomplete"
	"github.com/seamanticus/amitty/internal/ui/chat/attachmentspicker"
	"github.com/seamanticus/amitty/internal/ui/chat/channelspicker"
	"github.com/seamanticus/amitty/internal/ui/chat/composer"
	"github.com/seamanticus/amitty/internal/ui/chat/guildstree"
	"github.com/seamanticus/amitty/internal/ui/chat/joinprompt"
	"github.com/seamanticus/amitty/internal/ui/chat/memberstree"
	"github.com/seamanticus/amitty/internal/ui/chat/messageslist"
	"github.com/seamanticus/amitty/internal/ui/chat/pathprompt"
	"github.com/seamanticus/amitty/internal/ui/chat/profile"
	"github.com/seamanticus/amitty/internal/ui/chat/reactionspicker"
	"github.com/seamanticus/amitty/internal/ui/images"
)

type Model struct {
	// focused is the pane that receives keys.
	focused pane
	// guildsTreeVisible reports whether the guilds tree is shown left of the messages.
	guildsTreeVisible bool
	// membersTreeVisible reports whether the members tree is shown right of the messages when the selected channel has members.
	membersTreeVisible bool
	// width and pixelWidth are the terminal's width in columns and pixels, 0 until reported, which decide whether "auto" folds the members tree.
	width, pixelWidth int
	// overlay is the picker shown on top, which takes all input.
	overlay overlay

	guildsTree     guildstree.Model
	membersTree    memberstree.Model
	messagesList   messageslist.Model
	composer       composer.Model
	channelsPicker channelspicker.Model
	// attachmentsPicker picks an attachment or link of the selected message.
	attachmentsPicker attachmentspicker.Model
	// reactionsPicker picks an emoji to react to the selected message with.
	reactionsPicker reactionspicker.Model
	// pathPrompt asks for a file path to attach or to save to.
	pathPrompt pathprompt.Model
	// joinPrompt asks for an invite to join a server with; join is the join it leads to.
	joinPrompt joinprompt.Model
	join       joining
	// profile shows a member picked in the members tree.
	profile profile.Model

	selectedChannel *discord.Channel
	// windowUnfocused reports whether the terminal window lost focus.
	windowUnfocused bool

	state *ningen.State
	// events holds what the session sent, in order, until Update takes it; eventsCtx ends the wait for one.
	events    *handler.Queue
	eventsCtx context.Context
	// stopEvents ends the subscription feeding events, before the session closes.
	stopEvents func()
	// token is the one the session uses, so a different one in READY is a refresh to store.
	token string
	// browser keeps a new installation ID READY gives.
	browser *http.Browser

	// typers maps the users typing in the selected channel to when their typing indicator expires.
	typers map[discord.UserID]time.Time
	// outreach are when this session started new DMs or joined guilds, for their shared limit.
	outreach outreach

	// images are shared by the panes.
	images   map[discord.URL]tuiimage.Widget
	protocol imageProtocol
	// sentImages counts the images sent to the terminal or the application, which name them.
	sentImages int
	// Zero if the terminal does not report its cell size.
	cellWidth, cellHeight uint

	cfg *config.Config
}

// NewModel returns the chat of the account with token, whose requests and gateway identify as build and go through browser.
func NewModel(cfg *config.Config, build http.Build, browser *http.Browser, token string) Model {
	m := Model{
		browser: browser,
		typers:  make(map[discord.UserID]time.Time),
		images:  make(map[discord.URL]tuiimage.Widget),

		token: token,
		cfg:   cfg,
	}

	id := gateway.NewIdentifier(gateway.IdentifyCommand{
		Token:      token,
		Properties: http.IdentifyProperties(build),
		Capabilities: gateway.LazyUserNotes |
			gateway.VersionedReadStates |
			gateway.VersionedUserGuildSetttings |
			gateway.DedupeUserObjects |
			gateway.PrioritizedReadyPayload |
			gateway.MultipleGuildExperimentPopulations |
			gateway.NonChannelReadStates |
			gateway.AuthTokenRefresh |
			gateway.DebounceMessageReactions,
	})

	session := session.NewWithGateway(clientgateway.New(browser.Network(), id), browser.Client(token), handler.New())
	state := state.NewFromSession(session, defaultstore.New())
	m.state = ningen.FromState(state)

	m.events = handler.NewQueue()
	var cancel context.CancelFunc
	m.eventsCtx, cancel = context.WithCancel(context.Background())
	unsubscribe := m.state.Subscribe(m.events.Push)
	m.stopEvents = func() {
		unsubscribe()
		cancel()
	}
	m.state.StateLog = func(err error) {
		slog.Error("state log", "err", err)
	}
	m.state.OnRequest = append(m.state.OnRequest, httputil.WithHeaders(http.Headers(build)), onRequest)

	m.guildsTree = guildstree.NewModel(cfg, m.state)
	m.membersTree = memberstree.NewModel(cfg, m.state)
	media := http.NewMedia(browser.Network())
	m.messagesList = messageslist.NewModel(cfg, m.state, media)
	m.composer = composer.NewModel(cfg, m.state, media)
	m.channelsPicker = channelspicker.NewModel(cfg)
	m.attachmentsPicker = attachmentspicker.NewModel(cfg)
	m.reactionsPicker = reactionspicker.NewModel(cfg, m.state)

	m.guildsTreeVisible = cfg.Sidebar.Visible
	m.membersTreeVisible = cfg.MembersTree.Visible
	if m.guildsTreeVisible {
		// The guilds tree is focused first at start-up when visible.
		m.focused = paneGuildsTree
	} else {
		m.focused = paneMessagesList
	}
	m.syncPanes()
	return m
}

// overlay is the picker or prompt shown on top of the panes, if any.
type overlay int

const (
	overlayNone overlay = iota
	overlayChannelsPicker
	overlayAttachmentsPicker
	overlayReactionsPicker
	overlayPathPrompt
	overlayJoinPrompt
	overlayProfile
)

// pickerView returns the view of the picker or prompt shown on top, or nil if none is open.
func (m Model) pickerView() tui.Widget {
	switch m.overlay {
	case overlayChannelsPicker:
		return m.channelsPicker.View()
	case overlayAttachmentsPicker:
		return m.attachmentsPicker.View()
	case overlayReactionsPicker:
		return m.reactionsPicker.View()
	case overlayPathPrompt:
		return m.pathPrompt.View()
	case overlayJoinPrompt:
		return m.joinPrompt.View()
	case overlayProfile:
		return m.profile.View()
	case overlayNone:
		return nil
	}
	panic("invariant: unknown overlay")
}

// togglePicker opens the channels picker, or closes it if it is open.
func (m *Model) togglePicker() {
	if m.overlay == overlayChannelsPicker {
		m.closePicker()
	} else {
		m.openPicker()
	}
}

func (m *Model) openPicker() {
	m.overlay = overlayChannelsPicker
	m.channelsPicker.RefreshChannels(m.state)
}

func (m *Model) closePicker() {
	m.overlay = overlayNone
	m.channelsPicker.Reset()
}

// toggle shows or hides the side pane target and focuses it when shown.
func (m *Model) toggle(visible *bool, target pane) {
	*visible = !*visible
	if *visible {
		m.setFocus(target)
	} else if m.focused == target {
		m.focused = paneMessagesList
	}
}

// membersTreeShown reports whether the members tree is visible and the selected channel has members to show.
func (m Model) membersTreeShown() bool {
	return m.membersTreeVisible && m.membersTree.Shown()
}

// resized reports whether the terminal is narrower than ui.on_resize.is_less_than, which lays out the panes for a narrow terminal.
func (m Model) resized() bool {
	return m.cfg.UI.OnResize.IsLessThan.Narrower(m.width, m.pixelWidth)
}

// folded reports whether the members tree is drawn under the guilds tree rather than right of the messages.
func (m Model) folded() bool {
	switch m.cfg.MembersTree.Fold {
	case config.FoldOff:
		return false
	case config.FoldOn:
		return true
	case config.FoldAuto:
		return m.resized()
	}
	panic("invariant: unknown fold mode")
}

// cycleFocus focuses the next pane that can take the focus, going backwards if step is -1.
func (m *Model) cycleFocus(step pane) {
	next := m.focused
	for range paneCount - 1 {
		next = (next + step + paneCount) % paneCount
		if m.canFocus(next) {
			m.focused = next
			return
		}
	}
}

// syncPanes passes the panes the state of m that they show.
func (m *Model) syncPanes() {
	m.guildsTree.SetFocused(m.paneFocused(paneGuildsTree))
	m.messagesList.SetFocused(m.paneFocused(paneMessagesList))
	m.composer.SetFocused(m.paneFocused(paneComposer))
	m.membersTree.SetFocused(m.paneFocused(paneMembersTree))
	m.messagesList.SetFooter(m.typingFooter())
	m.messagesList.SetAvatarWidth(int(squareCols(uint(m.cfg.UI.MessagesList.Avatars.Height), m.imageCell())))
}

func (m Model) update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case gateway.Event:
		return m, cmd.Batch{m.applyEvent(msg), listen(m.eventsCtx, m.events)}
	case tui.FocusMsg:
		m.windowUnfocused = !msg.Focused
		return m, nil
	case guildstree.ChannelLoadedMsg:
		node := m.guildsTree.CurrentNode()
		if node == nil {
			return m, nil
		}
		channelID, ok := node.Reference().(discord.ChannelID)
		if !ok || channelID != msg.Channel.ID {
			return m, nil
		}

		membersCmd := m.setSelectedChannel(&msg.Channel)
		clear(m.typers)

		if m.cfg.AutoFocus {
			m.setFocus(paneComposer)
		}
		title := ui.ChannelToString(msg.Channel, m.cfg.Icons, m.state) + " - " + consts.Name
		// Not shown again soon.
		return m, cmd.Batch{m.clearImages(), cmd.SetTitle(title), m.messagesList.SetChannel(&msg.Channel, msg.Messages), membersCmd}
	case messageslist.Msg:
		return m, m.updatePane(paneMessagesList, msg)
	case tui.ResizeMsg:
		cell := m.imageCell()
		cols, rows := msg.Size()
		width, height := msg.PixelSize()
		m.width = cols
		// Kept as last reported, as a resize without pixels may be followed by the terminal's report of them.
		if width > 0 {
			m.pixelWidth = width
		}
		// A resize without a size in pixels may be followed by the terminal's own report of it.
		if cols > 0 && rows > 0 && width > 0 && height > 0 {
			m.cellWidth, m.cellHeight = uint(width/cols), uint(height/rows)
		}
		return m, m.reloadImages(cell)
	case ConfigChangedMsg:
		// Messages are cached as drawn, so they are drawn again in the new styles; avatars are loaded again cut to a new shape or size.
		m.messagesList.InvalidateRendered()
		m.syncPanes()
		if msg.Previous.HideDeletedUsers != m.cfg.HideDeletedUsers {
			m.guildsTree.RefreshPrivateChannels()
		}
		if msg.Previous.UI.MessagesList.Avatars != m.cfg.UI.MessagesList.Avatars {
			return m, cmd.Batch{m.clearImages(), m.messagesList.LoadImages()}
		}
		return m, nil
	case tui.TerminalInfoMsg:
		cell := m.imageCell()
		m.protocol = m.protocolFor(msg.Name)
		return m, m.reloadImages(cell)
	case images.RequestedMsg:
		command := m.loadImages(msg)
		return m, cmd.Batch{command, m.sendImages()}
	case images.LoadedMsg:
		// Another channel was selected, or the cell size changed, while it loaded.
		if _, ok := m.images[msg.URL]; !ok || msg.Cell != m.imageCell() {
			return m, nil
		}

		image, command := msg.Image, tui.Cmd(nil)
		id := m.sentImages%tuiimage.MaxID + 1
		switch m.protocol {
		case protocolHalfBlocks:
		case protocolKitty:
			image = image.Kitty(id).Compressor(newPooledCompressor)
		case protocolSixel:
			image = image.Sixel(id)
		}
		if command = image.Transmit(); command != nil {
			m.sentImages++
		}

		m.images[msg.URL] = image
		return m, cmd.Batch{command, m.sendImages()}
	case typingExpiredMsg:
		if until, ok := m.typers[msg.userID]; ok && !time.Now().Before(until) {
			delete(m.typers, msg.userID)
		}
		return m, nil
	case channelspicker.SelectedMsg:
		return m, m.navigateToChannel(msg.ChannelID)
	case channelspicker.CancelMsg:
		m.closePicker()
		return m, nil
	case attachmentspicker.SelectedMsg:
		m.overlay = overlayNone
		return m, msg.Action
	case attachmentspicker.CancelMsg:
		m.overlay = overlayNone
		return m, nil
	case messageslist.ShowAttachmentsMsg:
		m.attachmentsPicker.SetItems(picker.Items(msg))
		m.overlay = overlayAttachmentsPicker
		return m, nil
	case messageslist.ShowReactionsMsg:
		m.overlay = overlayReactionsPicker
		var command tui.Cmd
		m.reactionsPicker, command = m.reactionsPicker.Update(reactionspicker.ShowMsg(msg))
		return m, command
	case reactionspicker.SelectedMsg:
		m.overlay = overlayNone
		return m, m.react(msg)
	case reactionspicker.CancelMsg:
		m.overlay = overlayNone
		return m, nil
	case pathprompt.ShowMsg:
		var command tui.Cmd
		m.pathPrompt, command = pathprompt.New(m.cfg, msg)
		m.overlay = overlayPathPrompt
		return m, command
	case pathprompt.SubmittedMsg:
		m.overlay = overlayNone
		return m, msg.Cmd
	case pathprompt.CancelMsg:
		m.overlay = overlayNone
		return m, nil
	case profile.ShowMsg:
		var command tui.Cmd
		m.profile, command = profile.New(m.cfg, m.state, msg)
		m.overlay = overlayProfile
		return m, command
	case profile.ClosedMsg:
		m.overlay = overlayNone
		return m, nil
	case memberstree.OpenDMMsg:
		return m, m.openDM(msg.UserID, msg.Name)
	case startDMMsg:
		return m, m.startDM(msg.userID, time.Now())
	case dmStartedMsg:
		m.guildsTree.RefreshPrivateChannels()
		return m, m.navigateToChannel(msg.channelID)
	case guildstree.JoinMsg:
		m.openJoin()
		return m, nil
	case joinprompt.SubmittedMsg:
		// One join at a time: a second submit would send a second join.
		if m.join.invite.Code == "" {
			m.joinPrompt.SetStatus("Looking up the invite...")
			return m, lookUpInvite(m.state, msg.Input)
		}
		return m, nil
	case joinprompt.CancelMsg:
		return m, m.closeJoin()
	case inviteFoundMsg:
		if m.overlay != overlayJoinPrompt {
			return m, nil
		}
		m.joinPrompt.SetStatus("")
		return m, m.confirmJoin(msg.invite)
	case joinGuildMsg:
		if m.overlay != overlayJoinPrompt {
			return m, nil
		}
		return m, m.startJoin(msg.invite, time.Now())
	case joinFailedMsg:
		m.joinPrompt.SetStatus(msg.status)
		return m, m.endJoin()
	case guildJoinedMsg:
		return m, m.showJoined(msg.guild)
	case ui.CaptchaMsg[inviteCode], ui.CaptchaDoneMsg[inviteCode]:
		return m, m.updateJoin(msg)
	case messageslist.ReplyMsg:
		m.composer.StartReply(msg.Message, msg.Name, msg.Mention)
		m.setFocus(paneComposer)
		return m, nil
	case messageslist.EditMsg:
		m.composer.StartEdit(discord.Message(msg))
		m.setFocus(paneComposer)
		return m, nil
	case composer.SentMsg:
		m.messagesList.ShowNewest()
		return m, nil
	case QuitMsg:
		return m, closeState(m.state, m.stopEvents, StateClosedMsg{})
	case tui.KeyMsg:
		keybinds := m.cfg.Keybinds
		switch {
		case keybind.Matches(msg, keybinds.FocusGuildsTree.Keybind):
			m.setFocus(paneGuildsTree)
			return m, nil
		case keybind.Matches(msg, keybinds.FocusMembersTree.Keybind):
			m.setFocus(paneMembersTree)
			return m, nil
		case keybind.Matches(msg, keybinds.FocusMessagesList.Keybind):
			m.setFocus(paneMessagesList)
			return m, nil
		case keybind.Matches(msg, keybinds.FocusComposer.Keybind):
			m.setFocus(paneComposer)
			return m, nil

		case keybind.Matches(msg, keybinds.FocusPrevious.Keybind):
			m.cycleFocus(-1)
			return m, nil
		case keybind.Matches(msg, keybinds.FocusNext.Keybind):
			m.cycleFocus(1)
			return m, nil

		case keybind.Matches(msg, keybinds.ToggleGuildsTree.Keybind):
			m.toggle(&m.guildsTreeVisible, paneGuildsTree)
			return m, nil
		case keybind.Matches(msg, keybinds.ToggleMembersTree.Keybind):
			m.toggle(&m.membersTreeVisible, paneMembersTree)
			return m, nil
		case keybind.Matches(msg, keybinds.ToggleChannelsPicker.Keybind):
			m.togglePicker()
			return m, nil
		case keybind.Matches(msg, keybinds.Logout.Keybind):
			return m, ui.ShowDialog(
				"Are you sure you want to log out?",
				ui.DialogButton{Label: "No"},
				ui.DialogButton{Label: "Yes", Cmd: closeState(m.state, m.stopEvents, LogoutMsg{})},
			)
		}
	case composer.RefreshAutocompleteMsg, autocomplete.Msg:
		return m, m.updatePane(paneComposer, msg)
	case paneMsg:
		if msg.focus {
			m.setFocus(msg.pane)
		}
		if msg.msg == nil {
			return m, nil
		}
		return m, m.updatePane(msg.pane, msg.msg)
	}
	return m, m.route(msg)
}

// route sends msg to the open picker, which takes all input, and otherwise to the focused pane.
func (m *Model) route(msg tui.Msg) tui.Cmd {
	var cmd tui.Cmd
	switch m.overlay {
	case overlayChannelsPicker:
		m.channelsPicker, cmd = m.channelsPicker.Update(msg)
	case overlayAttachmentsPicker:
		m.attachmentsPicker, cmd = m.attachmentsPicker.Update(msg)
	case overlayReactionsPicker:
		m.reactionsPicker, cmd = m.reactionsPicker.Update(msg)
	case overlayPathPrompt:
		m.pathPrompt, cmd = m.pathPrompt.Update(msg)
	case overlayJoinPrompt:
		m.joinPrompt, cmd = m.joinPrompt.Update(msg)
	case overlayProfile:
		m.profile, cmd = m.profile.Update(msg)
	case overlayNone:
		cmd = m.updatePane(m.focused, msg)
	default:
		panic("invariant: unknown overlay")
	}
	return cmd
}

// updatePane updates pane target with msg.
func (m *Model) updatePane(target pane, msg tui.Msg) tui.Cmd {
	var cmd tui.Cmd
	switch target {
	case paneGuildsTree:
		m.guildsTree, cmd = m.guildsTree.Update(msg)
	case paneMessagesList:
		m.messagesList, cmd = m.messagesList.Update(msg)
	case paneComposer:
		m.composer, cmd = m.composer.Update(msg)
	case paneMembersTree:
		m.membersTree, cmd = m.membersTree.Update(msg)
	}
	return cmd
}

// paneWidget passes input to the widget of a pane and marks mouse input within the pane as meant for it.
type paneWidget struct {
	pane  pane
	child tui.Widget
}

func (p paneWidget) Size() (width, height layout.Length) {
	return p.child.Size()
}

func (p paneWidget) Layout(limits layout.Limits) layout.Size {
	return p.child.Layout(limits)
}

func (p paneWidget) Draw(screen tui.Screen, area tui.Rectangle) {
	p.child.Draw(screen, area)
}

func (p paneWidget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	out := p.child.Handle(msg, area)
	// Mouse input the pane leaves, but for a press, goes on to the other panes, for a slider dragged out of one to follow the pointer.
	if mouse, ok := msg.(tui.MouseMsg); ok && area.Contains(mouse.Position()) {
		pressed := mouse.Action == tui.MouseLeftDown
		if out != msg || pressed {
			return paneMsg{pane: p.pane, msg: out, focus: pressed}
		}
	}

	// Keys and paste reach only the focused pane, so what it makes of them goes to that pane through route.
	return out
}

// pane is one of the models shown side by side, one of which has the focus.
type pane int

const (
	paneGuildsTree pane = iota
	paneMessagesList
	paneComposer
	paneMembersTree
	paneCount
)

// paneView returns the view of target, marked so that clicking it focuses it.
func (m Model) paneView(target pane) tui.Widget {
	var child tui.Widget
	switch target {
	case paneGuildsTree:
		child = m.guildsTree.View()
	case paneMessagesList:
		child = m.messagesList.View()
	case paneComposer:
		child = m.composer.View()
	case paneMembersTree:
		child = m.membersTree.View()
	}
	return paneWidget{pane: target, child: child}
}

// paneFocused reports whether target has the focus and no picker is open.
func (m Model) paneFocused(target pane) bool {
	return m.focused == target && m.overlay == overlayNone
}

// setFocus focuses target if it can take the focus.
func (m *Model) setFocus(target pane) {
	if m.canFocus(target) {
		m.focused = target
	}
}

// canFocus reports whether target is shown and can take the focus.
func (m Model) canFocus(target pane) bool {
	switch target {
	case paneGuildsTree:
		return m.guildsTreeVisible
	case paneComposer:
		return !m.composer.Disabled()
	case paneMembersTree:
		return m.membersTreeShown()
	case paneMessagesList:
		return true
	}
	panic("invariant: unknown pane")
}

// typingFooter returns who is typing in the selected channel, or "" if nobody is.
func (m Model) typingFooter() string {
	channel := m.selectedChannel
	if channel == nil || len(m.typers) == 0 {
		return ""
	}

	var names []string
	for userID := range m.typers {
		if name := m.typerName(*channel, userID); name != "" {
			names = append(names, name)
		}
	}

	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0] + " is typing..."
	case 2:
		return fmt.Sprintf("%s and %s are typing...", names[0], names[1])
	case 3:
		return fmt.Sprintf("%s, %s, and %s are typing...", names[0], names[1], names[2])
	default:
		return "Several people are typing..."
	}
}

// typerName returns the name of userID in channel, or "" if they are not known there.
func (m Model) typerName(channel discord.Channel, userID discord.UserID) string {
	if channel.GuildID.IsValid() {
		member, err := m.state.Cabinet.Member(channel.GuildID, userID)
		if err != nil {
			return ""
		}

		return cmp.Or(member.Nick, member.User.DisplayName, member.User.Username)
	}

	index := slices.IndexFunc(channel.DMRecipients, func(recipient discord.User) bool { return recipient.ID == userID })
	if index == -1 {
		return ""
	}

	return channel.DMRecipients[index].DisplayOrUsername()
}

var _ tui.Model[Model] = Model{}

func (m Model) Init() tui.Cmd {
	return cmd.Batch{openState(m.state), listen(m.eventsCtx, m.events), m.guildsTree.Init(), m.messagesList.Init(), cmd.RequestTerminalInfo{}}
}

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	m, cmd := m.update(msg)
	m.syncPanes()
	return m, cmd
}

// View shows the panes, with the autocomplete over the messages and an open picker on top.
func (m Model) View() tui.Widget {
	picker := m.pickerView()

	messages := m.paneView(paneMessagesList)
	if popup := m.composer.AutocompleteView(); popup != nil {
		messages = stack.New(messages, popup)
	}

	var right tui.Widget = column.New(messages, m.paneView(paneComposer))

	main := right
	members := m.membersTreeShown()
	folded := members && m.folded()
	if members && !folded {
		width := m.cfg.MembersTree.WidthPercent
		main = row.New(
			column.New(main).Width(layout.FillPortion(100-width)),
			column.New(m.paneView(paneMembersTree)).Width(layout.FillPortion(width)),
		)
	}
	if m.guildsTreeVisible || folded {
		width := m.sidebarWidth()
		main = row.New(
			m.sidebarView(folded).Width(layout.FillPortion(width)),
			column.New(main).Width(layout.FillPortion(100-width)),
		)
	}

	if picker == nil {
		return main
	}
	if m.overlay != overlayJoinPrompt {
		picker = column.New(picker).Width(layout.Fixed(m.cfg.Picker.Width)).Height(layout.Fixed(m.cfg.Picker.Height))
	}
	return stack.New(
		// The panes behind the picker take no input.
		inert.New(main),
		backdrop.New().Style(m.cfg.Theme.Dialog.BackgroundStyle.Style),
		// The join prompt is as large as its content; the others take the picker's size.
		center.New(picker),
	)
}

// sidebarWidth returns the percentage of the width the sidebar takes, wider in a narrow terminal.
func (m Model) sidebarWidth() int {
	if m.resized() {
		return m.cfg.Sidebar.OnResize.WidthPercent
	}
	return m.cfg.Sidebar.WidthPercent
}

// sidebarView returns the guilds tree, if visible, over the members tree if folded; either fills the column alone.
func (m Model) sidebarView(folded bool) column.Widget {
	if !folded {
		return column.New(m.paneView(paneGuildsTree))
	}
	members := column.New(m.paneView(paneMembersTree))
	if !m.guildsTreeVisible {
		return members
	}
	height := m.cfg.MembersTree.FoldHeightPercent
	return column.New(
		column.New(m.paneView(paneGuildsTree)).Height(layout.FillPortion(100-height)),
		members.Height(layout.FillPortion(height)),
	)
}
