package messageslist

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui/chat/pathprompt"
)

// evidence: witnesses
func TestCreateFree(t *testing.T) {
	dir := t.TempDir()
	for _, tt := range []struct{ name, want string }{
		{"photo.png", "photo.png"},
		{"photo.png", "photo (1).png"},
		{"photo.png", "photo (2).png"},
		{".bashrc", ".bashrc"},
		{".bashrc", ".bashrc (1)"},
		{"archive.tar.gz", "archive.tar.gz"},
		{"archive.tar.gz", "archive.tar (1).gz"},
	} {
		file, err := createFree(dir, tt.name)
		if err != nil {
			t.Fatal(err)
		}
		if got := filepath.Base(file.Name()); got != tt.want {
			t.Errorf("createFree(%q) made %q, want %q", tt.name, got, tt.want)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// evidence: witnesses
func TestAttachmentName(t *testing.T) {
	for _, tt := range []struct {
		attachment discord.Attachment
		want       string
	}{
		{discord.Attachment{Filename: "a.png"}, "a.png"},
		{discord.Attachment{Filename: "../../etc/passwd"}, "passwd"},
		{discord.Attachment{Filename: "..", ContentType: "image/png"}, "attachment.png"},
		{discord.Attachment{}, "attachment"},
	} {
		if got := attachmentName(tt.attachment); got != tt.want {
			t.Errorf("attachmentName(%q) = %q, want %q", tt.attachment.Filename, got, tt.want)
		}
	}
}

func TestModelSaveAttachment(t *testing.T) {
	attachment := discord.Attachment{Filename: "a.png", ContentType: "image/png"}
	t.Run("ask starts the prompt in dir, creating it", func(t *testing.T) {
		ml := newTestModel(t, testConfig(t))
		dir := filepath.Join(t.TempDir(), "downloads")
		ml.cfg.Downloads = config.Downloads{Mode: config.DownloadAsk, Dir: dir}
		task, ok := ml.saveAttachment(attachment).(cmd.Task)
		if !ok {
			t.Fatal("not a task")
		}
		show, ok := task().(pathprompt.ShowMsg)
		if !ok || show.Path != filepath.Join(dir, "a.png") {
			t.Fatalf("prompt = %#v, want it at %s", show, filepath.Join(dir, "a.png"))
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("dir not created: %v", err)
		}
	})
	t.Run("ask with a picker runs it", func(t *testing.T) {
		ml := newTestModel(t, testConfig(t))
		ml.cfg.Downloads = config.Downloads{Mode: config.DownloadAsk, Dir: t.TempDir(), Picker: "fzf"}
		task, ok := ml.saveAttachment(attachment).(cmd.Task)
		if !ok {
			t.Fatal("not a task")
		}
		if _, ok := task().(cmd.Suspend); !ok {
			t.Fatal("the picker does not run with amitty suspended")
		}
	})
}
