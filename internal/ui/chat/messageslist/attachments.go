package messageslist

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/picker"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/chat/pathprompt"
)

// ShowAttachmentsMsg asks to pick one of the attachments and links of the selected message.
type ShowAttachmentsMsg picker.Items

func (m Model) open() tui.Cmd {
	return m.openWith(m.openAttachment)
}

func (m Model) openInBrowser() tui.Cmd {
	return m.openWith(func(attachment discord.Attachment) tui.Cmd { return openURL(attachment.URL) })
}

func (m Model) openWith(openAttachment func(discord.Attachment) tui.Cmd) tui.Cmd {
	selectedMessage, ok := m.selectedMessage()
	if !ok {
		return nil
	}

	urls := m.selectedURLs()
	switch total := len(urls) + len(selectedMessage.Attachments); {
	case total == 0:
		return nil
	case total > 1:
		return m.showAttachmentsList(urls, selectedMessage.Attachments, openAttachment)
	case len(urls) == 1:
		return openURL(urls[0])
	}

	return openAttachment(selectedMessage.Attachments[0])
}

func (m Model) download() tui.Cmd {
	selectedMessage, ok := m.selectedMessage()
	if !ok || len(selectedMessage.Attachments) == 0 {
		return nil
	}
	if len(selectedMessage.Attachments) == 1 {
		attachment := selectedMessage.Attachments[0]
		return m.confirmAttachment(attachment, m.saveAttachment(attachment))
	}

	items := make(picker.Items, len(selectedMessage.Attachments))
	for i, attachment := range selectedMessage.Attachments {
		items[i] = picker.Item{Content: text.New(attachment.Filename), FilterText: attachment.Filename, Reference: m.confirmAttachment(attachment, m.saveAttachment(attachment))}
	}
	return showAttachmentsPicker(items)
}

func (m Model) showAttachmentsList(urls []string, attachments []discord.Attachment, openAttachment func(discord.Attachment) tui.Cmd) tui.Cmd {
	var items picker.Items
	for _, attachment := range attachments {
		items = append(items, picker.Item{Content: text.New(attachment.Filename), FilterText: attachment.Filename, Reference: openAttachment(attachment)})
	}
	for _, url := range urls {
		items = append(items, picker.Item{Content: text.New(url), FilterText: url, Reference: openURL(url)})
	}
	return showAttachmentsPicker(items)
}

func showAttachmentsPicker(items picker.Items) tui.Cmd {
	return cmd.Task(func() tui.Msg { return ShowAttachmentsMsg(items) })
}

func (m Model) openAttachment(attachment discord.Attachment) tui.Cmd {
	return m.confirmAttachment(attachment, m.openDownloadedAttachment(attachment))
}

func (m Model) confirmAttachment(attachment discord.Attachment, action tui.Cmd) tui.Cmd {
	if !m.cfg.UI.MessagesList.Attachments.AllowedMIMETypes.Has(attachment.ContentType) {
		return ui.ShowDialog(
			"This attachment type is not allowed and may be unsafe. Continue anyway?",
			ui.DialogButton{Label: "No"},
			ui.DialogButton{Label: "Yes", Cmd: action},
		)
	}
	return action
}

// attachmentExtension returns the extension of the attachment's file name, or else one of its content type.
func attachmentExtension(attachment discord.Attachment) string {
	if extension := filepath.Ext(attachment.Filename); extension != "" {
		return extension
	}

	mediaType, _, _ := mime.ParseMediaType(attachment.ContentType)
	if extensions, _ := mime.ExtensionsByType(mediaType); len(extensions) != 0 {
		return extensions[0]
	}

	return ""
}

func (m Model) openDownloadedAttachment(attachment discord.Attachment) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		dir := filepath.Join(consts.CacheDir(), "attachments")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return ui.ErrorDialog("create attachments dir", err)
		}

		file, err := os.CreateTemp(dir, "attachment-*"+attachmentExtension(attachment))
		if err != nil {
			return ui.ErrorDialog("create attachment file", err)
		}
		path := file.Name()
		if err := file.Close(); err != nil {
			return ui.ErrorDialog("create attachment file", err)
		}

		if err := m.downloadAttachment(attachment, path); err != nil {
			return ui.ErrorDialog("download attachment", errors.Join(err, os.Remove(path)))
		}

		if err := ui.OpenExternal(context.Background(), path); err != nil {
			return ui.ErrorDialog("open attachment file", err)
		}

		return nil
	})
}

func (m Model) saveAttachment(attachment discord.Attachment) tui.Cmd {
	show := pathprompt.ShowMsg{
		Title: "Save as",
		Path:  filepath.Base(attachment.Filename),
		Submit: func(destination string) tui.Cmd {
			info, err := os.Stat(destination)
			if err == nil && info.IsDir() {
				destination = filepath.Join(destination, filepath.Base(attachment.Filename))
				_, err = os.Lstat(destination)
			}
			save := m.downloadTo(attachment, destination)
			if err == nil {
				return ui.ShowDialog(destination+" exists. Replace it?", ui.DialogButton{Label: "No"}, ui.DialogButton{Label: "Yes", Cmd: save})
			}
			return save
		},
	}
	return cmd.Task(func() tui.Msg { return show })
}

func (m Model) downloadTo(attachment discord.Attachment, destination string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if err := m.downloadAttachment(attachment, destination); err != nil {
			return ui.ErrorDialog("download attachment", err)
		}
		return nil
	})
}

func openURL(url string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if err := ui.OpenExternal(context.Background(), url); err != nil {
			return ui.ErrorDialog("open URL", err)
		}
		return nil
	})
}

// Get requests the file at url, by a new link to it if that one expired.
func (m Model) Get(url string) (*fhttp.Response, error) {
	resp, err := m.media.Get(context.Background(), url)
	if err != nil || resp.StatusCode == http.StatusOK {
		return resp, err
	}
	resp.Body.Close() //nolint:errcheck

	var links struct {
		URLs []struct {
			Refreshed string `json:"refreshed"`
		} `json:"refreshed_urls"`
	}
	body := map[string][]string{"attachment_urls": {url}}
	if err := m.state.RequestJSONInto(&links, http.MethodPost, api.Endpoint+"attachments/refresh-urls", httputil.WithJSONBody(body)); err != nil {
		return nil, err
	}

	if len(links.URLs) != 0 {
		if resp, err = m.media.Get(context.Background(), links.URLs[0].Refreshed); err != nil || resp.StatusCode == http.StatusOK {
			return resp, err
		}
		resp.Body.Close() //nolint:errcheck
	}

	return nil, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
}

func (m Model) downloadAttachment(attachment discord.Attachment, destination string) error {
	resp, err := m.Get(attachment.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, resp.Body)
	return errors.Join(err, file.Close())
}
