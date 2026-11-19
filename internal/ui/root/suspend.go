package root

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

func suspend() tui.Cmd {
	return cmd.Suspend(func() tui.Msg {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGCONT)
		defer signal.Stop(c)

		_ = syscall.Kill(0, syscall.SIGTSTP)
		<-c
		return nil
	})
}
