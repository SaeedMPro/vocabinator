// Package appdirs resolves platform-appropriate per-user directories.
package appdirs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appName = "vocabinator"

// DataDir returns the directory for persistent application data:
//
//	Linux:   $XDG_DATA_HOME/vocabinator or ~/.local/share/vocabinator
//	macOS:   ~/Library/Application Support/vocabinator
//	Windows: %LocalAppData%\vocabinator
func DataDir() (string, error) {
	return dataDir(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

// ConfigDir returns the directory for configuration files (os.UserConfigDir).
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(base, appName), nil
}

func dataDir(goos string, getenv func(string) string, home func() (string, error)) (string, error) {
	switch goos {
	case "windows":
		if dir := getenv("LocalAppData"); dir != "" {
			return filepath.Join(dir, appName), nil
		}
		return "", errors.New("resolve data directory: %LocalAppData% is not set")
	case "darwin":
		h, err := home()
		if err != nil {
			return "", fmt.Errorf("resolve data directory: %w", err)
		}
		return filepath.Join(h, "Library", "Application Support", appName), nil
	default:
		if dir := getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
			return filepath.Join(dir, appName), nil
		}
		h, err := home()
		if err != nil {
			return "", fmt.Errorf("resolve data directory: %w", err)
		}
		return filepath.Join(h, ".local", "share", appName), nil
	}
}
