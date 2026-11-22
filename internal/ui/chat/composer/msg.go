package composer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json/option"
	"github.com/seamanticus/amitty/internal/dmwire/utils/sendpart"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/autocomplete/emojis"
	"github.com/seamanticus/amitty/internal/ui/autocomplete/mentions"
	"github.com/seamanticus/amitty/internal/ui/chat/pathprompt"
	"golang.design/x/clipboard"
)

// RefreshAutocompleteMsg refreshes the autocomplete.
type RefreshAutocompleteMsg struct{}

// SentMsg reports that the composer sent a message.
type SentMsg struct{}

type editorMsg string

// edit is a text of the composer and the cursor in it.
type edit struct {
	text   string
	cursor int
}

// editMsg is the text and cursor that typing in the composer left.
type editMsg edit

type imagePastedMsg []byte

func pasteImage() tui.Cmd {
	return cmd.Task(func() tui.Msg {
		data, err := clipboard.Read(context.Background(), clipboard.FmtImage)
		if err != nil {
			slog.Error("failed to read from clipboard", "err", err)
			return nil
		}
		return imagePastedMsg(data)
	})
}

type filesPickedMsg struct {
	channelID discord.ChannelID
	files     []sendpart.File
}

func (m Model) pickFiles() tui.Cmd {
	selectedChannel := m.channel
	if selectedChannel == nil {
		return nil
	}
	channelID := selectedChannel.ID

	if picker := m.cfg.FilePicker; picker != "" {
		return ui.PickFiles(picker, "", func(paths []string) tui.Msg { return openPicked(channelID, paths) })
	}
	show := pathprompt.ShowMsg{
		Title: "Attach",
		Submit: func(path string) tui.Cmd {
			return cmd.Task(func() tui.Msg { return openPicked(channelID, []string{path}) })
		},
	}
	return cmd.Task(func() tui.Msg { return show })
}

// openPicked opens the files at paths to attach them in channelID, or reports the first that does not open, closing those opened.
func openPicked(channelID discord.ChannelID, paths []string) tui.Msg {
	files := make([]sendpart.File, 0, len(paths))
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			for _, opened := range files {
				closeFile(opened)
			}
			return ui.ErrorDialog("attach file", err)
		}
		files = append(files, sendpart.File{Name: filepath.Base(path), Reader: file})
	}
	return filesPickedMsg{channelID: channelID, files: files}
}

func closeFiles(files []sendpart.File) cmd.Task {
	return cmd.Task(func() tui.Msg {
		for _, file := range files {
			closeFile(file)
		}

		return nil
	})
}

func closeFile(file sendpart.File) {
	closer, ok := file.Reader.(io.Closer)
	if !ok {
		return
	}

	// The upload transport closes a file once it has sent it.
	if err := closer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		slog.Error("failed to close file", "err", err)
	}
}

func (m *Model) sendTyping() tui.Cmd {
	if !m.cfg.TypingIndicator.Send {
		return nil
	}

	now := time.Now()
	if now.Before(m.typingUntil) {
		return nil
	}
	m.typingUntil = now.Add(TypingDuration)

	selectedChannel := m.channel
	if selectedChannel == nil {
		return nil
	}
	channelID := selectedChannel.ID
	return cmd.Task(func() tui.Msg {
		if err := m.state.Typing(channelID); err != nil {
			slog.Error("failed to send typing indicator", "err", err)
		}
		return nil
	})
}

func (m *Model) send() tui.Cmd {
	selectedChannel := m.channel
	if selectedChannel == nil {
		return nil
	}

	text := strings.TrimSpace(m.text)
	if text == "" && len(m.sendMessageData.Files) == 0 {
		return nil
	}

	text = m.processText(selectedChannel, []byte(text))
	data := *m.sendMessageData
	data.AllowedMentions = m.replyAllowedMentions()
	data.Files = slices.Clone(data.Files)

	editing := m.editing
	m.typingUntil = time.Time{}
	m.reset()

	sent := cmd.Task(func() tui.Msg { return SentMsg{} })
	return cmd.Batch{sent, cmd.Task(func() tui.Msg {
		defer closeFiles(data.Files)()
		if editing != nil {
			m.editMessage(*editing, text)
			return nil
		}

		data.Content = text
		return m.sendMessage(selectedChannel.ID, data)
	})}
}

func (m Model) editMessage(message discord.Message, text string) {
	data := api.EditMessageData{Content: option.SomeNullable(text)}
	if _, err := m.state.EditMessageComplex(message.ChannelID, message.ID, data); err != nil {
		slog.Error("failed to edit message", "err", err)
	}
}

// sendMessage sends data to channelID and returns nil, or the dialog saying a message with files was not sent, as its files would be lost.
func (m Model) sendMessage(channelID discord.ChannelID, data api.SendMessageData) tui.Msg {
	if len(data.Files) > 0 {
		if err := sendWithFiles(context.Background(), m.state, m.upload, channelID, data, data.Files); err != nil {
			return ui.ErrorDialog("send the message with its files", err)
		}
		return nil
	}
	if _, err := m.state.SendMessageComplex(channelID, data); err != nil {
		slog.Error("failed to send message in channel", "channel_id", channelID, "err", err)
	}
	return nil
}

func (m *Model) complete() tui.Cmd {
	if item, ok := m.autocomplete.Selected(); ok {
		_, name := m.query()
		m.replace(m.cursor-len(name)-1, m.cursor, item.Insert()+" ")
	}

	return m.showCompletions("", nil)
}

func (m *Model) showAutocomplete() tui.Cmd {
	trigger, name := m.query()
	if trigger == 0 || m.channel == nil {
		return m.showCompletions("", nil)
	}

	guildID, limit := m.channel.GuildID, int(m.cfg.UI.Autocomplete.Limit)
	if trigger == '@' {
		items, loadImages := mentions.Items(m.cfg, m.state, m.channel, name, limit)
		return cmd.Batch{m.showCompletions(mentions.Title, items), loadImages, m.searchMember(guildID, name)}
	}

	items, loadImages := emojis.Items(m.cfg, m.state, guildID, name, limit)
	return cmd.Batch{m.showCompletions(emojis.Title, items), loadImages}
}

// searchMember asks Discord for members matching name; they arrive later as a gateway event.
func (m *Model) searchMember(guildID discord.GuildID, name string) tui.Cmd {
	if name == "" || !guildID.IsValid() {
		return nil
	}

	key := guildID.String() + " " + name
	if _, ok := m.memberSearchCache[key]; ok {
		return nil
	}
	// If searching for "ab" returns less than SearchLimit, then "abc" would not return anything new because we already searched everything starting with "ab".
	// This will still be true even if a new member joins because dmwire loads new members into the state.
	if count, ok := m.memberSearchCache[key[:len(key)-1]]; ok && count < m.state.MemberState.SearchLimit {
		m.memberSearchCache[key] = count
		return nil
	}

	now := time.Now()
	// Rate limit on our side because we can't distinguish between a successful search and SearchMember not doing anything because of its internal rate limit that we can't detect
	if m.lastSearch.Add(m.state.MemberState.SearchFrequency).After(now) {
		return nil
	}

	m.lastSearch = now
	nonce := memberSearchNonce + key
	return cmd.Task(func() tui.Msg {
		request := gateway.MembersByName([]discord.GuildID{guildID}, name, m.state.MemberState.SearchLimit)
		request.Presences = m.state.MemberState.RequestPresences
		request.Nonce = nonce
		if err := m.state.SendGateway(context.Background(), &request); err != nil {
			slog.Error("failed to search guild members", "err", err, "guild_id", guildID, "query", name)
		}
		return nil
	})
}

func (m Model) openEditor() tui.Cmd {
	if m.cfg.Editor == "" {
		return cmd.Task(func() tui.Msg {
			slog.Warn("Attempt to open file with editor, but no editor is set")
			return nil
		})
	}
	text, cfg := m.text, m.cfg
	return cmd.Suspend(func() tui.Msg {
		return runEditor(cfg, text)
	})
}

// runEditor opens text in the editor and returns what it was edited to.
func runEditor(cfg *config.Config, text string) tui.Msg {
	file, err := os.CreateTemp("", tmpFilePattern)
	if err != nil {
		slog.Error("failed to create tmp file", "err", err)
		return nil
	}

	name := file.Name()
	defer func() {
		if err := os.Remove(name); err != nil {
			slog.Error("failed to remove tmp file", "err", err)
		}
	}()

	_, _ = file.WriteString(text)
	_ = file.Close()

	cmd, err := cfg.EditorCommand(name)
	if err != nil {
		slog.Error("failed to create editor command", "err", err)
		return nil
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("failed to run command", "args", cmd.Args, "err", err)
		return nil
	}

	edited, err := os.ReadFile(name)
	if err != nil {
		slog.Error("failed to read tmp file", "name", name, "err", err)
		return nil
	}

	return editorMsg(strings.TrimSpace(string(edited)))
}
