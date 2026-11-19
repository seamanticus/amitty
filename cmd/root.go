package cmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/logger"
	"github.com/seamanticus/amitty/internal/tokenstore"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/ui/root"
)

// buildTimeout bounds the startup fetch of the build number.
const buildTimeout = 10 * time.Second

// decryptTimeout bounds the startup decrypt of the token, which may wait for a passphrase.
const decryptTimeout = 2 * time.Minute

func Run() error {
	configPath := flag.String("config-path", config.DefaultPath(), "path of the configuration file")
	logPath := flag.String("log-path", logger.DefaultPath(), "path of the log file")
	logLevel := flag.String("log-level", "info", "log level: raw-debug, debug, info, warn or error; raw-debug logs every API response and gateway event in full, tokens included")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s\n", buildVersion())
		return nil
	}

	level, err := logger.ParseLevel(*logLevel)
	if err != nil {
		return err
	}
	ws.EnableRawEvents = level <= slog.LevelDebug

	logFile, err := logger.Load(*logPath, level)
	if err != nil {
		return fmt.Errorf("failed to load logger: %w", err)
	}
	defer logFile.Close() //nolint:errcheck
	// A crash in any goroutine leaves the terminal unreadable, so its trace is kept in the log too.
	if err := debug.SetCrashOutput(logFile, debug.CrashOptions{}); err != nil {
		return fmt.Errorf("crash output: %w", err)
	}

	if wrote, err := config.WriteDefault(*configPath); err != nil {
		return err
	} else if wrote {
		slog.Info("wrote the default config", "path", *configPath)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	network, err := http.ParseNetwork(cfg.Proxy)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	slog.Info("network", "via", network)

	build := fetchBuild(network)

	store, token, err := loadToken(cfg)
	if err != nil {
		return err
	}

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

	return tui.NewApplication(root.NewModel(cfg, build, network, store, token), tui.WithScreen(screen)).Run()
}

// loadToken opens the token store of cfg and decrypts its token before the screen starts, so a passphrase prompt owns the terminal. An empty token means none is stored.
func loadToken(cfg *config.Config) (tokenstore.Store, string, error) {
	dir, err := consts.StateDir()
	if err != nil {
		return tokenstore.Store{}, "", err
	}
	store, err := tokenstore.New(filepath.Join(dir, tokenstore.FileName), cfg.TokenEncryptCmd, cfg.TokenDecryptCmd)
	if err != nil {
		return tokenstore.Store{}, "", fmt.Errorf("token store: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), decryptTimeout)
	defer cancel()
	token, ok, err := store.Load(ctx)
	if err != nil {
		// A token that exists but does not decrypt is not deleted: the passphrase or key may come back.
		return tokenstore.Store{}, "", fmt.Errorf("%w (delete %s to log in again)", err, store.Path())
	}
	if !ok {
		slog.Info("no stored token; showing the login", "path", store.Path())
	}
	return store, token, nil
}

// fetchBuild fetches the current build number before the screen starts, falling back to the generated one with a warning on stderr and in the log.
func fetchBuild(network http.Network) http.Build {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	build, err := http.FetchBuild(ctx, network)
	if err != nil {
		fallback := http.Build{}
		slog.Warn("failed to fetch the build number; using the generated one", "err", err, "build", fallback.Number())
		fmt.Fprintf(os.Stderr, "warning: %v; using build %d\n", err, fallback.Number())
		return fallback
	}
	slog.Info("fetched the build number", "build", build.Number())
	return build
}
