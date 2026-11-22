package messageslist

import (
	"slices"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/markdown"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	tuiimage "github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/list"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/scrollbar"
	"github.com/seamanticus/amitty/internal/tui/spacer"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/tint"
	"github.com/seamanticus/amitty/internal/tui/viewport"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/images"
	"github.com/yuin/goldmark/v2/ast"
)

type messageItem struct {
	message discord.Message
	// view is the message as drawn, keeping its wrapped text, or nil until it is shown and after what it shows changes.
	view tui.Widget
	// separator is the line with the date of the message after it, or nil for a message.
	separator tui.Widget
	// grouped reports whether the message is drawn without a header, under that of the message before it.
	grouped bool
	// card numbers the message's card, a message with those grouped under it, counting up as messages arrive; older pages count down, so a shown card keeps its number and shade. numbered is false until it is set.
	card     int
	numbered bool

	// root is the parsed markdown of the message's content, or nil until it is needed and after the text is invalidated, as it resolves mentions when parsed.
	root ast.Node
}

type Model struct {
	// cursor is the selected item, or -1 for none, and target the item selected last, which stays in view once deselected.
	cursor, target int
	scrollState    viewport.ScrollState

	cfg   *config.Config
	state *ningen.State
	// media fetches attachments and images.
	media dhttp.Media
	// channel is the selected channel, or nil for none.
	channel *discord.Channel
	items   []messageItem

	renderer *markdown.Renderer
	// images are the chat model's, which loads them, by URL.
	images map[discord.URL]tuiimage.Widget
	// focused, footer and avatarWidth, in cells, are set by the chat model, which keeps what they follow from.
	focused     bool
	footer      string
	avatarWidth int
}

func NewModel(cfg *config.Config, state *ningen.State, media dhttp.Media) Model {
	ml := Model{
		media:    media,
		cursor:   -1,
		target:   -1,
		cfg:      cfg,
		state:    state,
		renderer: markdown.NewRenderer(cfg),
	}

	ml.scrollState.ScrollToEnd()
	return ml
}

// SetFocused sets whether the list takes keys.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// SetFooter sets what is shown under the messages, such as who is typing.
func (m *Model) SetFooter(footer string) {
	m.footer = footer
}

// SetAvatarWidth sets how many cells wide avatars are, which the list leaves for them before they load.
func (m *Model) SetAvatarWidth(width int) {
	if width != m.avatarWidth {
		m.avatarWidth = width
		m.clearViews()
	}
}

// title returns the selected channel and its topic, or "Messages" when none is selected.
func (m Model) title() string {
	if m.channel == nil {
		return "Messages"
	}
	title := ui.ChannelToString(*m.channel, m.cfg.Icons, m.state)
	if topic := m.channel.Topic; topic != "" {
		title += " - " + topic
	}
	return title
}

func (m Model) listView() viewport.Widget {
	cfg, theme := m.cfg.UI.ScrollBar, m.cfg.Theme.ScrollBar
	bar := scrollbar.New().
		SymbolSet(cfg.SymbolSet.SymbolSet).
		Style(theme.TrackStyle.Style).
		ThumbStyle(theme.ThumbStyle.Style)
	// The model moves the selection itself, so the list is not focused and only takes clicks.
	l := list.New(m.cursor, len(m.items), m.buildItem).
		// A group's messages have no spacing between them.
		Spacing(func(index int) int {
			if m.items[index].grouped {
				return 0
			}
			return int(m.cfg.UI.MessagesList.Spacing)
		}).
		SelectedStyle(m.cfg.Theme.MessagesList.SelectedMessageStyle.Style).
		OnChange(func(cursor int) tui.Msg { return listMsg(cursor) })
	return viewport.New(l, m.scrollState).
		Target(l.Target(m.target)).
		ScrollBar(bar, cfg.Visibility.ScrollBarVisibility).
		// Older messages are added above, so the position is kept from the newest, which also keeps the newest in view.
		AnchorBottom().
		Keybind(ui.ScrollKeybind(m.cfg.Keybinds.MessagesList.ScrollKeybinds)).
		Focused(m.focused).
		OnChange(func(s *viewport.ScrollState) tui.Msg { return scrollMsg(*s) })
}

// setCursor selects the item at index and scrolls to it if it changed, or selects none if it is negative.
func (m *Model) setCursor(index int) {
	if index >= 0 && index != m.cursor {
		m.scrollState.ScrollToTarget()
		m.target = index
	}

	m.cursor = max(index, -1)
}

// SetChannel shows messages of channel, newest first as Discord sends them, scrolled to the newest, requests the members who wrote them, and fetches the page of older messages before them, as the web client does on opening a channel.
func (m *Model) SetChannel(channel *discord.Channel, messages []discord.Message) tui.Cmd {
	m.channel = channel
	m.reset()
	m.setMessages(messages)
	if channel.GuildID.IsValid() {
		return cmd.Batch{m.requestGuildMembers(channel.GuildID, messages), m.LoadImages(), m.fetchOlderMessages()}
	}
	return cmd.Batch{m.LoadImages(), m.fetchOlderMessages()}
}

// ShowNewest clears the selection and scrolls to the newest message, which then stays in view as messages arrive until the user scrolls up.
func (m *Model) ShowNewest() {
	m.clearSelection()
	m.scrollState.ScrollToEnd()
}

// UpdateMessage replaces the shown message with the same ID.
func (m *Model) UpdateMessage(message discord.Message) tui.Cmd {
	m.setMessage(m.indexOf(message.ChannelID, message.ID), message)
	return m.LoadImages()
}

// RefreshMessage shows the cached state of a message, such as after a reaction.
func (m *Model) RefreshMessage(channelID discord.ChannelID, id discord.MessageID) tui.Cmd {
	message, err := m.state.Cabinet.Message(channelID, id)
	if err != nil {
		return nil
	}

	m.setMessage(m.indexOf(channelID, id), *message)
	return m.LoadImages()
}

// DeleteMessage removes the shown message with id.
func (m *Model) DeleteMessage(channelID discord.ChannelID, id discord.MessageID) {
	m.deleteMessage(m.indexOf(channelID, id))
}

// indexOf returns the index of the message with id in the channel, or -1 if it is not shown.
func (m Model) indexOf(channelID discord.ChannelID, id discord.MessageID) int {
	if m.channel == nil || m.channel.ID != channelID {
		return -1
	}
	return slices.IndexFunc(m.items, func(item messageItem) bool { return item.separator == nil && item.message.ID == id })
}

func (m *Model) clearViews() {
	for i := range m.items {
		m.items[i].view = nil
	}
}

func (m *Model) reset() {
	m.items = nil
	m.cursor, m.target = -1, -1
	m.scrollState = viewport.ScrollState{}
	m.scrollState.ScrollToEnd()
}

func (m *Model) setMessages(messages []discord.Message) {
	m.items = make([]messageItem, 0, len(messages))
	for _, message := range slices.Backward(messages) {
		m.items = append(m.items, messageItem{message: message})
	}
	m.rebuildItems()
}

func (m *Model) AddMessage(message discord.Message) tui.Cmd {
	// Appending keeps the items before it as they are, so only the separator of a new day is added rather than all rebuilt.
	var previous *discord.Message
	card := -1
	if last := len(m.items) - 1; last >= 0 {
		previous = &m.items[last].message
		card = m.items[last].card
	}

	separator := m.shouldSeparate(previous, message)
	if separator {
		card = m.lastCard()
		m.items = append(m.items, messageItem{separator: m.separatorView(message)})
	}

	grouped := !separator && m.shouldGroup(previous, message)
	if !grouped {
		card++
	}
	m.items = append(m.items, messageItem{message: message, grouped: grouped, card: card, numbered: true})
	return m.LoadImages()
}

// lastCard returns the card of the newest message, or -1 for none.
func (m Model) lastCard() int {
	for _, item := range slices.Backward(m.items) {
		if item.separator == nil {
			return item.card
		}
	}
	return -1
}

// alternate reports whether the message of item takes the alternate style, as its card is every alternate_every-th.
func (m Model) alternate(item *messageItem) bool {
	every := int(m.cfg.UI.MessagesList.AlternateEvery)
	// Older pages number their cards below zero, where Go's % is negative, so the remainder is taken up to 0..every-1.
	return every > 0 && (item.card%every+every)%every == every-1
}

func (m *Model) setMessage(index int, message discord.Message) {
	if index < 0 || index >= len(m.items) || m.items[index].separator != nil {
		return
	}

	m.items[index] = messageItem{message: message}
	m.rebuildItems()
}

func (m *Model) deleteMessage(index int) {
	if index < 0 || index >= len(m.items) || m.items[index].separator != nil {
		return
	}

	cursor := m.cursor
	if cursor == index {
		cursor = m.messageIndex(index, -1)
		if cursor == -1 {
			cursor = m.messageIndex(index, 1)
		}
	}
	if cursor > index {
		cursor--
	}
	m.items = slices.Delete(m.items, index, index+1)
	m.setCursor(cursor)
	m.rebuildItems()
}

func (m *Model) clearSelection() {
	m.setCursor(-1)
}

func (m Model) buildItem(index int) tui.Widget {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	item := &m.items[index]
	if item.separator != nil {
		return item.separator
	}

	if item.view == nil {
		item.view = m.itemView(item)
	}
	// The selection merges under what the card draws, so the alternate tint would cover it on the selected card.
	if alternate := m.cfg.Theme.MessagesList.AlternateStyle.Style; index != m.cursor && m.alternate(item) && alternate != tcell.StyleDefault {
		return tint.New(item.view, alternate)
	}
	return item.view
}

// itemView returns the message of item with its header or avatar, embeds, previews, reactions and what it replies to.
func (m Model) itemView(item *messageItem) tui.Widget {
	message := &item.message
	root := m.contentRoot(item)
	style := m.cfg.Theme.MessagesList.MessageStyle.Style

	// Attachments are named in the text, before the embeds.
	body := column.New(messageText(m.renderMessage(*message, root, style, item.grouped))).Height(layout.Shrink)
	for _, embed := range m.embedViews(*message, root) {
		body = body.Push(embed)
	}

	for _, source := range m.previewSources(*message) {
		if preview, ok := m.images[source.proxy]; ok {
			body = body.Push(preview)
		}
	}

	if m.cfg.UI.MessagesList.Reactions.Enabled && !m.blocked(message.Author) {
		body = body.Push(m.reactionsView(message.Reactions))
	}

	var view tui.Widget = body
	if m.showsAvatar(message.Author) {
		// The avatar's column is left empty before it is requested, if it fails to load, and under the avatar of a group, so the text does not shift.
		var gutter tui.Widget = spacer.New().Width(layout.Fixed(m.avatarWidth))
		if avatar, ok := m.images[avatarURL(message.Author)]; ok && !item.grouped {
			if width, _ := avatar.Size(); width != layout.Fixed(0) {
				gutter = avatar
			}
		}
		view = row.New(gutter, body).Spacing(1).Height(layout.Shrink)
	}

	if message.Type == discord.InlinedReplyMessage && !m.blocked(message.Author) {
		builder := new(markdown.Builder)
		m.drawReply(builder, *message, style)
		var reply tui.Widget = messageText(builder.Finish())
		// The quote's background fills its line; a tint around the card merges under it, so the quote keeps its own shade.
		if bg := m.cfg.Theme.MessagesList.ReplyStyle.GetBackground(); bg != tcell.ColorDefault {
			reply = tint.New(reply, tcell.StyleDefault.Background(bg))
		}
		view = column.New(reply, view).Height(layout.Shrink)
	}
	return view
}

// shouldGroup reports whether message is drawn under the header of previous, the message before it, which is nil for the first.
func (m Model) shouldGroup(previous *discord.Message, message discord.Message) bool {
	cfg := m.cfg.UI.MessagesList.Grouping
	return cfg.Enabled && previous != nil &&
		// Replies and system messages start a group, as their header says more than who wrote them.
		message.Type == discord.DefaultMessage &&
		(previous.Type == discord.DefaultMessage || previous.Type == discord.InlinedReplyMessage) &&
		// Webhooks post under one ID with a name of each message's own.
		message.Author.ID == previous.Author.ID && message.Author.Username == previous.Author.Username &&
		message.Timestamp.Time().Sub(previous.Timestamp.Time()) <= cfg.Interval
}

// separatorDatePadding is on either side of the date on its separator line.
const separatorDatePadding = " "

// separatorView returns the line with the date of message in the middle of it. It is made once, when the separator is added, as the list builds every item each time it is laid out.
func (m Model) separatorView(message discord.Message) tui.Widget {
	date := message.Timestamp.Time().In(time.Local).Format(m.cfg.DateSeparator.Format)
	style := m.cfg.Theme.MessagesList.MessageStyle.Dim(true)
	return box.New(spacer.New()).
		Borders(tui.BordersTop).
		BorderSet(tui.BorderSet{Top: m.cfg.DateSeparator.Character}).
		BorderStyle(style).
		// The separator is kept, so its date is laid out once.
		Title(text.New(separatorDatePadding + date + separatorDatePadding).Style(style).Wrapping(text.WrappingNone).Paragraph(new(text.Paragraph))).
		Height(layout.Fixed(1))
}

// rebuildItems replaces date separators and groups while retaining message views and selection.
// Cards are renumbered so the newest message numbered before keeps its number, as messages are only added around those shown; a message whose shade changes is drawn again.
func (m *Model) rebuildItems() {
	items := make([]messageItem, 0, len(m.items))
	// before holds each message as it was, by its index in items; separators are left zero.
	before := make([]messageItem, 0, len(m.items))
	cursor, target := -1, -1
	var previous *discord.Message
	card, shift := -1, 0
	for index, item := range m.items {
		if item.separator != nil {
			continue
		}

		separator := m.shouldSeparate(previous, item.message)
		if separator {
			items = append(items, messageItem{separator: m.separatorView(item.message)})
			before = append(before, messageItem{})
		}
		// A message that joins or leaves a group is drawn again, with or without its header.
		if grouped := !separator && m.shouldGroup(previous, item.message); grouped != item.grouped {
			item.grouped, item.view = grouped, nil
		}
		if !item.grouped {
			card++
		}
		if item.numbered {
			shift = item.card - card
		}

		if index == m.cursor {
			cursor = len(items)
		}

		if index == m.target {
			target = len(items)
		}

		before = append(before, item)
		item.card = card
		items = append(items, item)
		previous = &item.message
	}

	for i := range items {
		item := &items[i]
		if item.separator != nil {
			continue
		}
		item.card += shift
		if was := before[i]; was.numbered && m.alternate(&was) != m.alternate(item) {
			item.view = nil
		}
		item.numbered = true
	}
	m.items = items
	// The selected and target messages only move with the separators, so they are not scrolled to.
	m.cursor, m.target = cursor, target
}

// shouldSeparate reports whether a date separator goes before message, as it is the first (previous is nil) or starts a new day.
func (m Model) shouldSeparate(previous *discord.Message, message discord.Message) bool {
	return m.cfg.DateSeparator.Enabled && (previous == nil || !sameLocalDate(previous.Timestamp, message.Timestamp))
}

func sameLocalDate(first, second discord.Timestamp) bool {
	firstTime, secondTime := first.Time().In(time.Local), second.Time().In(time.Local)
	return firstTime.Year() == secondTime.Year() && firstTime.YearDay() == secondTime.YearDay()
}

// messageIndex finds the next message in direction (-1 or 1), excluding start.
func (m Model) messageIndex(start, direction int) int {
	for index := start + direction; index >= 0 && index < len(m.items); index += direction {
		if m.items[index].separator == nil {
			return index
		}
	}

	return -1
}

func (m *Model) onRowCursorChanged(index int) {
	if index < 0 || index >= len(m.items) || m.items[index].separator == nil {
		return
	}
	target := m.messageIndex(index, -1)
	if target == -1 {
		target = m.messageIndex(index, 1)
	}
	m.setCursor(target)
}

func (m Model) selectedMessage() (*discord.Message, bool) {
	cursor := m.cursor
	if cursor < 0 || cursor >= len(m.items) || m.items[cursor].separator != nil {
		return nil, false
	}

	return &m.items[cursor].message, true
}

// selectedURLs returns the web links of the selected message, from its saved markdown.
func (m Model) selectedURLs() []string {
	if message, ok := m.selectedMessage(); ok {
		return m.messageURLs(*message, m.contentRoot(&m.items[m.cursor]))
	}
	return nil
}

// prependOlder shows the older messages loaded above those of the same channel.
func (m *Model) prependOlder(msg olderMessagesLoadedMsg) tui.Cmd {
	selectedChannel := m.channel
	if selectedChannel == nil || selectedChannel.ID != msg.channelID {
		return nil
	}
	// A second fetch of the same page, as the prefetch on opening and a scroll to the top can make, is already shown.
	if first := m.messageIndex(-1, 1); first >= 0 && len(msg.older) > 0 && msg.older[len(msg.older)-1].ID >= m.items[first].message.ID {
		return nil
	}

	older := make([]messageItem, len(msg.older))
	for index, message := range msg.older {
		older[index] = messageItem{message: message}
	}

	m.items = slices.Concat(older, m.items)
	// The selected and target messages stay the same, so they are not scrolled to.
	if m.cursor >= 0 {
		m.cursor += len(older)
	}

	if m.target >= 0 {
		m.target += len(older)
	}

	m.rebuildItems()
	if selectedChannel.GuildID.IsValid() {
		return cmd.Batch{m.requestGuildMembers(selectedChannel.GuildID, msg.older), m.LoadImages()}
	}

	return m.LoadImages()
}

// scrolledToTop reports whether view is scrolled to the oldest message shown, which is false while the messages fit, as there is nothing to scroll.
func scrolledToTop(view viewport.Viewport) bool {
	return view.Content().Height > view.Bounds().Height && view.AbsoluteOffsetReversed().Y == 0
}

// atTop reports whether the oldest message shown is selected.
func (m Model) atTop() bool {
	return m.cursor >= 0 && m.cursor == m.messageIndex(-1, 1)
}

func (m *Model) selectDown() {
	if m.cursor == -1 {
		m.selectBottom()
	} else if next := m.messageIndex(m.cursor, 1); next >= 0 {
		m.setCursor(next)
	}
}

func (m *Model) selectTop() tui.Cmd {
	if m.atTop() {
		return nil
	}
	m.setCursor(m.messageIndex(-1, 1))
	return m.fetchOlderMessages()
}

func (m *Model) selectBottom() {
	m.setCursor(m.messageIndex(len(m.items), -1))
}

func (m *Model) selectReply() {
	messages := m.items
	cursor := m.cursor
	if cursor < 0 || cursor >= len(messages) {
		return
	}

	ref := messages[cursor].message.ReferencedMessage
	if ref == nil {
		return
	}

	index := slices.IndexFunc(messages, func(item messageItem) bool {
		return item.separator == nil && item.message.ID == ref.ID
	})
	if index != -1 {
		m.setCursor(index)
	}
}

// isEditable reports whether item is a message of the user in channelID with text content they can edit.
func (m Model) isEditable(item messageItem, channelID discord.ChannelID) bool {
	message := item.message
	isText := message.Type == discord.DefaultMessage || message.Type == discord.InlinedReplyMessage
	return item.separator == nil && message.ChannelID == channelID && ui.IsMe(m.state, message.Author.ID) &&
		message.ID.IsValid() && message.Content != "" && len(message.MessageSnapshots) == 0 && isText
}

func (m Model) canDeleteMessage(message discord.Message) bool {
	return ui.IsMe(m.state, message.Author.ID) ||
		(message.GuildID.IsValid() && m.state.HasPermissions(message.ChannelID, discord.PermissionManageMessages))
}

// canReact reports whether the user may react to messages in the channel, which is always so in DMs.
// Reacting with an emoji nobody has reacted with takes adding reactions as well as reading the message history.
func (m Model) canReact() bool {
	return m.channel != nil && (!m.channel.GuildID.IsValid() || m.state.HasPermissions(m.channel.ID, discord.PermissionAddReactions|discord.PermissionReadMessageHistory))
}

func (m Model) InvalidateRendered() {
	for index := range m.items {
		m.items[index].view, m.items[index].root = nil, nil
	}
}

// contentRoot returns the parsed markdown of the content of the item's message, parsing it the first time, or nil if markdown is disabled.
func (m Model) contentRoot(item *messageItem) ast.Node {
	if item.root == nil && m.cfg.Markdown.Enabled {
		item.root = m.parse(item.message)
	}
	return item.root
}

var _ tui.Model[Model] = Model{}

func (Model) Init() tui.Cmd { return nil }

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case images.ChangedMsg:
		m.images = msg
		m.clearViews()
		return m, nil
	case tui.KeyMsg:
		keybinds := m.cfg.Keybinds.MessagesList
		switch {
		case keybind.Matches(msg, keybinds.Cancel.Keybind):
			m.clearSelection()
		case keybind.Matches(msg, keybinds.SelectUp.Keybind):
			return m, m.selectUp()
		case keybind.Matches(msg, keybinds.SelectDown.Keybind):
			m.selectDown()
		case keybind.Matches(msg, keybinds.SelectTop.Keybind):
			return m, m.selectTop()
		case keybind.Matches(msg, keybinds.SelectBottom.Keybind):
			m.selectBottom()
		case keybind.Matches(msg, keybinds.SelectReply.Keybind):
			m.selectReply()
		case keybind.Matches(msg, keybinds.YankID.Keybind):
			return m, m.yankMessageID()
		case keybind.Matches(msg, keybinds.YankContent.Keybind):
			return m, m.yankContent()
		case keybind.Matches(msg, keybinds.YankURL.Keybind):
			return m, m.yankURL()
		case keybind.Matches(msg, keybinds.Open.Keybind):
			return m, m.open()
		case keybind.Matches(msg, keybinds.OpenInBrowser.Keybind):
			return m, m.openInBrowser()
		case keybind.Matches(msg, keybinds.Download.Keybind):
			return m, m.download()
		case keybind.Matches(msg, keybinds.React.Keybind):
			return m, m.showReactions()
		case keybind.Matches(msg, keybinds.Reply.Keybind):
			return m, m.reply(false)
		case keybind.Matches(msg, keybinds.ReplyMention.Keybind):
			return m, m.reply(true)
		case keybind.Matches(msg, keybinds.Edit.Keybind):
			return m, m.editSelectedMessage()
		case keybind.Matches(msg, keybinds.Delete.Keybind):
			return m, m.deleteSelectedMessage()
		case keybind.Matches(msg, keybinds.DeleteConfirm.Keybind):
			return m, m.confirmDelete()
		}
	case olderMessagesLoadedMsg:
		return m, m.prependOlder(msg)
	case scrollMsg:
		m.scrollState = viewport.ScrollState(msg)
		// Older messages are fetched once as the list is scrolled to the top, rather than on each scroll there.
		if !scrolledToTop(m.scrollState.Origin()) && scrolledToTop(m.scrollState.Viewport()) {
			return m, m.fetchOlderMessages()
		}
	case listMsg:
		top := m.atTop()
		m.cursor, m.target = int(msg), int(msg)
		m.onRowCursorChanged(m.cursor)
		if !top && m.atTop() {
			return m, m.fetchOlderMessages()
		}
	}
	return m, nil
}

func (m Model) View() tui.Widget {
	return ui.Box(m.listView(), m.cfg, m.focused).
		Title(ui.Label(m.title(), m.cfg.Theme.Title, m.focused)).
		Footer(ui.Label(m.footer, m.cfg.Theme.Footer, m.focused))
}
