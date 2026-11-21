package joinprompt

import (
	"path/filepath"
	"testing"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

func TestViewFitsItsContent(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	limits := layout.Limits{Max: layout.Size{Width: 200, Height: 50}}
	size := New(cfg).View().Layout(limits)
	// The input, the side padding and the two borders; then the title row, the top padding, input, gap, status and bottom border.
	wantWidth, wantHeight := inputWidth+2*paddingSide+2, 1+paddingTop+3+cfg.UI.Border.Padding[1]+1
	if size.Width != wantWidth || size.Height != wantHeight {
		t.Errorf("size = %+v, want %dx%d", size, wantWidth, wantHeight)
	}
}
