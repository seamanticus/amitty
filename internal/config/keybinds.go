package config

import (
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

type Keybind struct {
	keybind.Keybind
}

var _ toml.Unmarshaler = (*Keybind)(nil)

func (k *Keybind) UnmarshalTOML(value any) error {
	var keys []string
	switch value := value.(type) {
	case string:
		keys = []string{value}
	case []any:
		keys = stringValues(value)
	default:
		return nil
	}

	bound := keybind.New(keys...)
	// Keep displayed help key aligned with configured key(s).
	help := k.Help()
	if keys := bound.Keys(); len(keys) > 0 {
		help.Key = keys[0]
	}

	k.Keybind = bound.WithHelp(help.Key, help.Desc)
	return nil
}

// stringValues returns the strings in values, skipping anything else.
func stringValues(values []any) []string {
	var keys []string
	for _, value := range values {
		if key, ok := value.(string); ok {
			keys = append(keys, key)
		}
	}

	return keys
}

// MarshalTOML writes the keys as UnmarshalTOML reads them: one key as a string, an unbound keybind as an empty one, and several keys as a list.
func (k Keybind) MarshalTOML() ([]byte, error) {
	keys := k.Keys()
	if len(keys) <= 1 {
		return []byte(strconv.Quote(strings.Join(keys, ""))), nil
	}

	quoted := make([]string, len(keys))
	for index, key := range keys {
		quoted[index] = strconv.Quote(key)
	}
	return []byte("[" + strings.Join(quoted, ", ") + "]"), nil
}

// Global returns the keybinds that act wherever the focus is.
func (k Keybinds) Global() []keybind.Keybind {
	return []keybind.Keybind{
		k.ToggleGuildsTree.Keybind, k.ToggleMembersTree.Keybind, k.ToggleChannelsPicker.Keybind,
		k.ToggleHelp.Keybind, k.ToggleFullHelp.Keybind, k.Suspend.Keybind,
		k.FocusGuildsTree.Keybind, k.FocusMembersTree.Keybind, k.FocusMessagesList.Keybind, k.FocusComposer.Keybind,
		k.FocusPrevious.Keybind, k.FocusNext.Keybind,
		k.Settings.Keybind, k.Logout.Keybind, k.Quit.Keybind,
	}
}

// key builds a Keybind with desc as its help description, bound to keys, the first of which is shown in help. It is unbound if that is empty.
func key(desc string, keys ...string) Keybind {
	return Keybind{keybind.New(keys...).WithHelp(keys[0], desc)}
}

type ScrollKeybinds struct {
	// Scroll without changing the selection.
	ScrollUp     Keybind `toml:"scroll_up"`
	ScrollDown   Keybind `toml:"scroll_down"`
	ScrollTop    Keybind `toml:"scroll_top"`
	ScrollBottom Keybind `toml:"scroll_bottom"`
}

type SelectionKeybinds struct {
	SelectUp     Keybind `toml:"select_up"`
	SelectDown   Keybind `toml:"select_down"`
	SelectTop    Keybind `toml:"select_top"`
	SelectBottom Keybind `toml:"select_bottom"`
}

type PickerKeybinds struct {
	SelectionKeybinds
	Select Keybind `toml:"select"`
	Cancel Keybind `toml:"cancel"`
}

type PathPromptKeybinds struct {
	// Highlight the previous or next matching entry; up from the first goes back to the typed text.
	SelectUp   Keybind `toml:"select_up"`
	SelectDown Keybind `toml:"select_down"`
	// Insert the highlighted entry, or extend the name typed by what every matching entry shares.
	Complete Keybind `toml:"complete"`
	// Open the highlighted directory, or submit the highlighted file or the path typed.
	Submit Keybind `toml:"submit"`
	Cancel Keybind `toml:"cancel"`
}

type ProfileKeybinds struct {
	YankID Keybind `toml:"yank_id"`
	Close  Keybind `toml:"close"`
}

type TreeKeybinds struct {
	ScrollKeybinds
	// Scroll sideways to read names that are cut off.
	ScrollLeft  Keybind `toml:"scroll_left"`
	ScrollRight Keybind `toml:"scroll_right"`

	SelectionKeybinds
	// Select the highlighted text-based channel, or expand or collapse the highlighted node.
	SelectCurrent Keybind `toml:"select_current"`

	YankID Keybind `toml:"yank_id"`

	CollapseAll        Keybind `toml:"collapse_all"`
	CollapseParentNode Keybind `toml:"collapse_parent_node"`
	MoveToParentNode   Keybind `toml:"move_to_parent_node"`
}

type GuildsTreeKeybinds struct {
	TreeKeybinds
	// Highlight the previous or next unread guild or channel, wrapping around.
	SelectPreviousUnread Keybind `toml:"select_previous_unread"`
	SelectNextUnread     Keybind `toml:"select_next_unread"`
	// Accept or reject the highlighted message request, a DM from someone you are not friends with; each asks first.
	AcceptRequest Keybind `toml:"accept_request"`
	RejectRequest Keybind `toml:"reject_request"`
}

type MembersTreeKeybinds struct {
	// select_current on a member opens your DM with them.
	TreeKeybinds
	// Show the highlighted member's profile.
	ViewProfile Keybind `toml:"view_profile"`
}

type MessagesListKeybinds struct {
	SelectionKeybinds
	ScrollKeybinds

	// Select the message that the selected message replies to.
	SelectReply Keybind `toml:"select_reply"`
	// Reply to the selected message.
	Reply Keybind `toml:"reply"`
	// Reply (with mention) to the selected message.
	ReplyMention Keybind `toml:"reply_mention"`

	Cancel Keybind `toml:"cancel"`
	Edit   Keybind `toml:"edit"`

	// Delete without confirmation. Unbound by default.
	Delete        Keybind `toml:"delete"`
	DeleteConfirm Keybind `toml:"delete_confirm"`

	// Open attachments with their associated application. MIME types outside the
	// allowlist require confirmation. Hyperlinks still use the browser.
	Open Keybind `toml:"open"`
	// Open attachments or hyperlinks in the browser.
	OpenInBrowser Keybind `toml:"open_in_browser"`
	// React to the selected message with an emoji picked from a list.
	React Keybind `toml:"react"`
	// Download one of the selected message's attachments.
	Download Keybind `toml:"download"`

	// Yank (copy) the selected message's content/url/id.
	YankContent Keybind `toml:"yank_content"`
	YankURL     Keybind `toml:"yank_url"`
	YankID      Keybind `toml:"yank_id"`
}

type ComposerKeybinds struct {
	// Edit your newest editable message loaded in the current channel, with no age limit.
	// Only when the composer is empty, with no reply, attachments, or edit in progress.
	EditLast Keybind `toml:"edit_last"`
	// Paste from clipboard (supports both text and images).
	Paste Keybind `toml:"paste"`
	Send  Keybind `toml:"send"`
	// Insert a new line at the cursor.
	Newline Keybind `toml:"newline"`
	// Remove existing text or cancel reply.
	Cancel Keybind `toml:"cancel"`
	// Complete usernames when mentioning.
	TabComplete Keybind `toml:"tab_complete"`
	// Toggle whether a reply mentions its author.
	ToggleReplyMention Keybind `toml:"toggle_reply_mention"`
	Undo               Keybind `toml:"undo"`

	OpenEditor     Keybind `toml:"open_editor"`
	OpenFilePicker Keybind `toml:"open_file_picker"`

	// Move the cursor. Readline's keys, with shift added to those other keybinds take.
	// Shift with ctrl is told apart from ctrl alone only in terminals with an enhanced keyboard protocol, such as kitty's.
	CursorLeft  Keybind `toml:"cursor_left"`
	CursorRight Keybind `toml:"cursor_right"`
	CursorUp    Keybind `toml:"cursor_up"`
	CursorDown  Keybind `toml:"cursor_down"`
	LineStart   Keybind `toml:"line_start"`
	LineEnd     Keybind `toml:"line_end"`
	// Move over a word, a run of letters and digits, and what is before it.
	WordLeft  Keybind `toml:"word_left"`
	WordRight Keybind `toml:"word_right"`

	// Delete text. Readline's keys, with shift added to those other keybinds take.
	DeleteBackward Keybind `toml:"delete_backward"`
	DeleteForward  Keybind `toml:"delete_forward"`
	// Delete what word_left and word_right move over.
	DeleteWordBackward Keybind `toml:"delete_word_backward"`
	DeleteWordForward  Keybind `toml:"delete_word_forward"`
	// Delete back to the whitespace before the cursor.
	DeleteToSpaceBackward Keybind `toml:"delete_to_space_backward"`
	// Delete to the start or end of the line, or the newline there if the cursor is at it.
	// Readline deletes to the start of the line with ctrl+u, which is undo here, so it is on ctrl+j.
	DeleteToLineStart Keybind `toml:"delete_to_line_start"`
	DeleteToLineEnd   Keybind `toml:"delete_to_line_end"`
}

type AutocompleteKeybinds struct {
	SelectionKeybinds
}

type Keybinds struct {
	// Hide/show the guilds tree.
	ToggleGuildsTree Keybind `toml:"toggle_guilds_tree"`
	// Hide/show the members tree.
	ToggleMembersTree    Keybind `toml:"toggle_members_tree"`
	ToggleChannelsPicker Keybind `toml:"toggle_channels_picker"`
	// Hide/show keybind help.
	ToggleHelp Keybind `toml:"toggle_help"`
	// Switch between compact and full keybind help.
	ToggleFullHelp Keybind `toml:"toggle_full_help"`
	Suspend        Keybind `toml:"suspend"`

	FocusGuildsTree   Keybind `toml:"focus_guilds_tree"`
	FocusMembersTree  Keybind `toml:"focus_members_tree"`
	FocusMessagesList Keybind `toml:"focus_messages_list"`
	FocusComposer     Keybind `toml:"focus_composer"`

	// Cycle focus between the widgets.
	FocusPrevious Keybind `toml:"focus_previous"`
	FocusNext     Keybind `toml:"focus_next"`

	Picker PickerKeybinds `toml:"picker"`
	// Only while asked for a file path, to attach or to save to.
	PathPrompt PathPromptKeybinds `toml:"path_prompt"`
	// Only while a profile is shown, opened with enter on a member.
	Profile ProfileKeybinds `toml:"profile"`
	// Only while focusing on the guilds tree.
	GuildsTree GuildsTreeKeybinds `toml:"guilds_tree"`
	// Only while focusing on the members tree.
	MembersTree MembersTreeKeybinds `toml:"members_tree"`
	// Only while focusing on sent messages.
	MessagesList MessagesListKeybinds `toml:"messages_list"`
	// Only while typing a message.
	Composer ComposerKeybinds `toml:"composer"`
	// Only while the autocomplete is shown.
	Autocomplete AutocompleteKeybinds `toml:"autocomplete"`

	// Log out and delete the stored token.
	// Requires re-login upon restart.
	Logout Keybind `toml:"logout"`
	Quit   Keybind `toml:"quit"`
	// Open or close the settings panel, which edits this file. Most terminals send ctrl+/ as ctrl+_.
	Settings Keybind `toml:"settings"`
}

func selectionKeybinds(up, down []string, top, bottom string) SelectionKeybinds {
	return SelectionKeybinds{
		SelectUp:     key("up", up...),
		SelectDown:   key("down", down...),
		SelectTop:    key("top", top),
		SelectBottom: key("btm", bottom),
	}
}

func scrollKeybinds(up, down, top, bottom string) ScrollKeybinds {
	return ScrollKeybinds{
		ScrollUp:     key("scr up", up),
		ScrollDown:   key("scr down", down),
		ScrollTop:    key("scr top", top),
		ScrollBottom: key("scr btm", bottom),
	}
}

func defaultKeybinds() Keybinds {
	vi := selectionKeybinds([]string{"k", "up"}, []string{"j", "down"}, "g", "G")
	emacs := selectionKeybinds([]string{"ctrl+p"}, []string{"ctrl+n"}, "home", "end")
	tree := TreeKeybinds{
		SelectionKeybinds: vi,
		ScrollKeybinds:    scrollKeybinds("pgup", "pgdn", "home", "end"),
		ScrollLeft:        key("scr left", "h"),
		ScrollRight:       key("scr right", "l"),
		SelectCurrent:     key("select", "enter"),
		YankID:            key("copy id", "i"),

		CollapseAll:        key("collapse all", "_"),
		CollapseParentNode: key("collapse parent", "-"),
		MoveToParentNode:   key("parent", "p"),
	}
	return Keybinds{
		ToggleGuildsTree:     key("toggle guilds", "alt+g"),
		ToggleMembersTree:    key("toggle members", "alt+y"),
		ToggleChannelsPicker: key("channels picker", "ctrl+k"),
		ToggleHelp:           key("toggle help", "alt+."),
		ToggleFullHelp:       key("full help", "ctrl+."),
		Suspend:              key("suspend", "ctrl+z"),

		FocusGuildsTree:   key("guilds", "ctrl+g"),
		FocusMembersTree:  key("members", "ctrl+y"),
		FocusMessagesList: key("messages", "ctrl+t"),
		FocusComposer:     key("composer", "ctrl+i"),

		FocusPrevious: key("focus prev", "ctrl+h", "ctrl+left", "ctrl+up"),
		FocusNext:     key("focus next", "ctrl+l", "ctrl+right", "ctrl+down"),

		Picker: PickerKeybinds{
			// The query is one line, so the arrows are free to move the selection.
			SelectionKeybinds: selectionKeybinds([]string{"ctrl+p", "up"}, []string{"ctrl+n", "down"}, "home", "end"),
			Select:            key("sel", "enter"),
			Cancel:            key("cancel", "esc"),
		},
		PathPrompt: PathPromptKeybinds{
			SelectUp:   key("up", "up", "ctrl+p"),
			SelectDown: key("down", "down", "ctrl+n"),
			Complete:   key("complete", "tab"),
			Submit:     key("ok", "enter"),
			Cancel:     key("cancel", "esc"),
		},
		Profile: ProfileKeybinds{
			YankID: key("yank id", "y"),
			Close:  key("close", "esc", "q"),
		},
		GuildsTree: GuildsTreeKeybinds{
			TreeKeybinds:         tree,
			SelectPreviousUnread: key("prev unread", "K"),
			SelectNextUnread:     key("next unread", "J"),
			AcceptRequest:        key("accept", "a"),
			RejectRequest:        key("reject", "r"),
		},
		MembersTree: MembersTreeKeybinds{TreeKeybinds: tree, ViewProfile: key("profile", "v")},
		MessagesList: MessagesListKeybinds{
			SelectionKeybinds: vi,
			ScrollKeybinds:    scrollKeybinds("K", "J", "home", "end"),

			SelectReply:  key("sel reply", "s"),
			Reply:        key("reply", "R"),
			ReplyMention: key("@reply", "r"),

			Cancel: key("cancel", "esc"),
			Edit:   key("edit", "e"),

			Delete:        key("force delete", ""),
			DeleteConfirm: key("delete", "d"),

			Open:          key("open", "o"),
			OpenInBrowser: key("open browser", "O"),
			React:         key("react", "a"),
			Download:      key("download", "D"),

			YankContent: key("copy text", "y"),
			YankURL:     key("copy url", "u"),
			YankID:      key("copy id", "i"),
		},
		Composer: ComposerKeybinds{
			EditLast:           key("edit last", "up"),
			Paste:              key("paste", "ctrl+v"),
			Send:               key("send", "enter"),
			Newline:            key("nl", "shift+enter"),
			Cancel:             key("cancel", "esc"),
			TabComplete:        key("complete", "tab"),
			ToggleReplyMention: key("toggle @reply", "shift+tab"),
			Undo:               key("undo", "ctrl+u"),

			OpenEditor:     key("editor", "ctrl+e"),
			OpenFilePicker: key("attach", "ctrl+\\"),

			CursorLeft:  key("left", "ctrl+b"),
			CursorRight: key("right", "ctrl+f"),
			CursorUp:    key("up", "ctrl+p"),
			CursorDown:  key("down", "ctrl+n"),
			LineStart:   key("line start", "ctrl+a"),
			LineEnd:     key("line end", "ctrl+shift+e"),
			WordLeft:    key("word left", "alt+b"),
			WordRight:   key("word right", "alt+f"),

			DeleteBackward:        key("del back", "ctrl+shift+h"),
			DeleteForward:         key("del", "ctrl+shift+d"),
			DeleteWordBackward:    key("del word back", "alt+backspace"),
			DeleteWordForward:     key("del word", "alt+shift+d"),
			DeleteToSpaceBackward: key("del to space", "ctrl+w"),
			DeleteToLineStart:     key("del to line start", "ctrl+j"),
			DeleteToLineEnd:       key("del to line end", "ctrl+shift+k"),
		},
		Autocomplete: AutocompleteKeybinds{SelectionKeybinds: emacs},

		Logout:   key("logout", "ctrl+d"),
		Settings: key("settings", "ctrl+/", "ctrl+_"),
		Quit:     key("quit", "ctrl+c"),
	}
}
