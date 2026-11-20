package messageslist

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/seamanticus/amitty/internal/config"
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
	return m.openWith(func(attachment discord.Attachment) tui.Cmd { return m.openURL(attachment.URL) })
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
		return m.openURL(urls[0])
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
		items = append(items, picker.Item{Content: text.New(url), FilterText: url, Reference: m.openURL(url)})
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

		if err := m.downloadAttachment(attachment, func() (*os.File, error) { return os.Create(path) }); err != nil {
			return ui.ErrorDialog("download attachment", errors.Join(err, os.Remove(path)))
		}

		return ui.Open(m.cfg.Openers, attachment.ContentType, path, "open attachment file")
	})
}

func (m Model) saveAttachment(attachment discord.Attachment) tui.Cmd {
	downloads := m.cfg.Downloads
	dir, name := consts.DownloadDir(downloads.Dir), attachmentName(attachment)
	return cmd.Task(func() tui.Msg {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ui.ErrorDialog("create downloads dir", err)
		}
		switch downloads.Mode {
		case config.DownloadToDir:
			return m.downloadInto(attachment, dir, name)
		case config.DownloadAsk:
			if downloads.Picker != "" {
				return ui.PickFiles(downloads.Picker, dir, func(paths []string) tui.Msg {
					path := paths[0]
					if !filepath.IsAbs(path) {
						path = filepath.Join(dir, path)
					}
					return m.saveTo(attachment, path)
				})
			}
			return pathprompt.ShowMsg{
				Title:  "Save as",
				Path:   filepath.Join(dir, name),
				Submit: func(destination string) tui.Cmd { return m.saveTo(attachment, destination) },
			}
		default:
			panic("invariant: unknown download mode")
		}
	})
}

// attachmentName returns the attachment's file name without any directory, or a generic one if it has none.
func attachmentName(attachment discord.Attachment) string {
	switch name := filepath.Base(attachment.Filename); name {
	case ".", "..", string(filepath.Separator):
		return "attachment" + attachmentExtension(attachment)
	default:
		return name
	}
}

// saveTo saves the attachment at destination, or in it under its own name if it is a directory, asking before it replaces a file.
func (m Model) saveTo(attachment discord.Attachment, destination string) tui.Cmd {
	info, err := os.Stat(destination)
	if err == nil && info.IsDir() {
		destination = filepath.Join(destination, attachmentName(attachment))
		_, err = os.Lstat(destination)
	}
	save := m.downloadTo(attachment, destination)
	if err == nil {
		return ui.ShowDialog(destination+" exists. Replace it?", ui.DialogButton{Label: "No"}, ui.DialogButton{Label: "Yes", Cmd: save})
	}
	return save
}

func (m Model) downloadTo(attachment discord.Attachment, destination string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if err := m.downloadAttachment(attachment, func() (*os.File, error) { return os.Create(destination) }); err != nil {
			return ui.ErrorDialog("download attachment", err)
		}
		return nil
	})
}

// downloadInto saves the attachment in dir as name, or as the first free "name (n)" if it is taken, and returns the error dialog if it fails.
func (m Model) downloadInto(attachment discord.Attachment, dir, name string) tui.Msg {
	var path string
	create := func() (*os.File, error) {
		file, err := createFree(dir, name)
		if file != nil {
			path = file.Name()
		}
		return file, err
	}
	if err := m.downloadAttachment(attachment, create); err != nil {
		if path != "" {
			err = errors.Join(err, os.Remove(path))
		}
		return ui.ErrorDialog("download attachment", err)
	}
	return nil
}

// maxNumbered bounds the numbered names createFree tries.
const maxNumbered = 1000

// createFree creates name in dir, or the first of "name (1)" to "name (maxNumbered)" that does not exist, with the number before the extension.
func createFree(dir, name string) (*os.File, error) {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	if stem == "" {
		// A dotfile such as ".bashrc" is all stem.
		stem, extension = name, ""
	}
	candidate := name
	for n := 1; ; n++ {
		file, err := os.OpenFile(filepath.Join(dir, candidate), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			return file, err
		}
		if n > maxNumbered {
			return nil, fmt.Errorf("%s and %d numbered names are taken in %s", name, maxNumbered, dir)
		}
		candidate = fmt.Sprintf("%s (%d)%s", stem, n, extension)
	}
}

// openURL opens url with the url opener, or xdg-open.
func (m Model) openURL(url string) tui.Cmd {
	return ui.Open(m.cfg.Openers, config.OpenerURL, url, "open URL")
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

// downloadAttachment writes the attachment to the file create makes once the response arrives, so a failed request leaves no file.
func (m Model) downloadAttachment(attachment discord.Attachment, create func() (*os.File, error)) error {
	resp, err := m.Get(attachment.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	file, err := create()
	if err != nil {
		return err
	}
	_, err = io.Copy(file, resp.Body)
	return errors.Join(err, file.Close())
}
