package cmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ayn2op/arikawa/v3/utils/ws"
	"github.com/ayn2op/tview"
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/discordo/internal/config"
	"github.com/seamanticus/discordo/internal/http"
	"github.com/seamanticus/discordo/internal/logger"
	"github.com/seamanticus/discordo/internal/ui/root"
)

// buildTimeout bounds the startup fetch of the build number.
const buildTimeout = 10 * time.Second

func Run() error {
	configPath := flag.String("config-path", config.DefaultPath(), "path of the configuration file")
	logPath := flag.String("log-path", logger.DefaultPath(), "path of the log file")
	logLevel := flag.String("log-level", "info", "log level")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s\n", buildVersion())
		return nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	ws.EnableRawEvents = level == slog.LevelDebug

	logFile, err := logger.Load(*logPath, level)
	if err != nil {
		return fmt.Errorf("failed to load logger: %w", err)
	}
	defer logFile.Close() //nolint:errcheck

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	build := fetchBuild()

	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create screen: %w", err)
	}

	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to init screen: %w", err)
	}

	if cfg.Mouse {
		screen.EnableMouse()
	}
	screen.EnablePaste()
	if cfg.Notifications.WhenUnfocused {
		screen.EnableFocus()
	}

	return tview.NewApplication(root.NewModel(cfg, build), tview.WithScreen(screen)).Run()
}

// fetchBuild fetches the current build number before the screen starts, falling back to the generated one with a warning on stderr and in the log.
func fetchBuild() http.Build {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	build, err := http.FetchBuild(ctx)
	if err != nil {
		fallback := http.Build{}
		slog.Warn("failed to fetch the build number; using the generated one", "err", err, "build", fallback.Number())
		fmt.Fprintf(os.Stderr, "warning: %v; using build %d\n", err, fallback.Number())
		return fallback
	}
	slog.Info("fetched the build number", "build", build.Number())
	return build
}
