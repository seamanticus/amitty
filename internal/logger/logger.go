package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/seamanticus/amitty/internal/consts"
)

const fileName = "logs.txt"

func DefaultPath() string {
	return filepath.Join(consts.CacheDir(), fileName)
}

// Load opens the log file and configures default logger.
func Load(path string, level slog.Level) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(file, opts)
	slog.SetDefault(slog.New(handler))
	return file, nil
}
