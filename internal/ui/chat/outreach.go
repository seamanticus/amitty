package chat

import (
	"strconv"
	"time"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/ui"
)

// outreachLimit and outreachWindow bound how many new DMs and guild joins amitty makes, together, as both are what Discord's spam filter watches. The count is kept in memory only.
const (
	outreachLimit  = 3
	outreachWindow = time.Hour
)

// outreach are the times new DMs were started or guilds joined, oldest first.
type outreach []time.Time

// allow reports whether another may happen at now, recording it if so, and keeps only those within the window.
func (o *outreach) allow(now time.Time) bool {
	kept := (*o)[:0]
	for _, at := range *o {
		if now.Sub(at) < outreachWindow {
			kept = append(kept, at)
		}
	}
	*o = kept
	if len(kept) >= outreachLimit {
		return false
	}
	*o = append(kept, now)
	return true
}

// outreachRefused is the dialog shown when the limit is reached.
func outreachRefused() tui.Cmd {
	return ui.ShowDialog("You started "+strconv.Itoa(outreachLimit)+" new DMs or joined guilds in the last hour. Try again later.", ui.DialogButton{Label: "OK"})
}
