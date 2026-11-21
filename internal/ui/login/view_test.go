package login

import (
	"strings"
	"testing"

	"github.com/rockorager/go-uucode"
	"github.com/seamanticus/amitty/internal/discordtest"
)

func TestLogo(t *testing.T) {
	// proof: exhaustive
	lines := strings.Split(logo, "\n")
	if len(lines) != logoHeight {
		t.Fatalf("logo has %d rows, want %d", len(lines), logoHeight)
	}
	for i, line := range lines {
		if width := uucode.StringWidth(line); width != logoWidth || len([]rune(line)) != logoWidth {
			t.Errorf("row %d is %d cells and %d runes, want %d of each", i, width, len([]rune(line)), logoWidth)
		}
	}
}

func TestModelView(t *testing.T) {
	// evidence: witnesses
	for _, tt := range []struct {
		name          string
		width, height int
		logo          bool
	}{
		{"a large screen shows the logo", 100, 30, true},
		{"a short screen leaves it out", 100, 16, false},
		{"a narrow screen leaves it out", logoWidth - 1, 30, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			drawn := discordtest.Draw(t, newModel(t).View(), tt.width, tt.height)
			if got := strings.Contains(drawn, "▜███▙"); got != tt.logo {
				t.Errorf("logo drawn = %v:\n%s", got, drawn)
			}
			for _, want := range []string{methodsTitle, "Password │ QR │ Token", "Email or phone number : ", "Password : ", "│ Log in │"} {
				if !strings.Contains(drawn, want) {
					t.Errorf("%q not drawn:\n%s", want, drawn)
				}
			}
		})
	}
}
